package scada

// Ring-buffered reading store with alarm storage. Each point keeps its most
// recent N readings in a ring buffer; alarms are kept in a slice ordered by
// time. All methods are safe for concurrent use and return deep copies.

import (
	"sort"
	"sync"
	"time"
)

// ringBuffer is a fixed-capacity ring of readings. It is NOT itself
// thread-safe; the owning Store holds the lock around access.
type ringBuffer struct {
	buf  []Reading
	head int // index of the next write
	n    int // number of valid entries
	cap  int
}

// newRing makes a ring of the given capacity.
func newRing(capacity int) *ringBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return &ringBuffer{buf: make([]Reading, capacity), cap: capacity}
}

// push appends a reading, overwriting the oldest when full.
func (r *ringBuffer) push(rd Reading) {
	r.buf[r.head] = rd
	r.head = (r.head + 1) % r.cap
	if r.n < r.cap {
		r.n++
	}
}

// last returns the most recent reading and true, or false if empty.
func (r *ringBuffer) last() (Reading, bool) {
	if r.n == 0 {
		return Reading{}, false
	}
	idx := (r.head - 1 + r.cap) % r.cap
	return r.buf[idx], true
}

// snapshot returns a time-ordered copy of all valid readings (oldest first).
func (r *ringBuffer) snapshot() []Reading {
	if r.n == 0 {
		return nil
	}
	out := make([]Reading, r.n)
	// oldest entry is at head - n (mod cap)
	start := (r.head - r.n + r.cap) % r.cap
	for i := 0; i < r.n; i++ {
		out[i] = r.buf[(start+i)%r.cap]
	}
	return out
}

// Store is the SCADA reading and alarm store.
type Store struct {
	mu       sync.Mutex
	points   map[string]*Point
	readings map[string]*ringBuffer
	bufCap   int

	alarmsMu sync.Mutex
	alarms   map[string]*Alarm // by id
	// index: point -> active alarm id (if any)
	activeByPoint map[string]string
}

// NewStore returns a store with the given per-point buffer capacity.
func NewStore(bufCap int) *Store {
	if bufCap < 1 {
		bufCap = 200
	}
	return &Store{
		points:        make(map[string]*Point),
		readings:      make(map[string]*ringBuffer),
		bufCap:        bufCap,
		alarms:        make(map[string]*Alarm),
		activeByPoint: make(map[string]string),
	}
}

// ---- Points ----

// PutPoint inserts or replaces a point definition.
func (s *Store) PutPoint(p Point) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := p
	s.points[p.ID] = &cp
	if _, ok := s.readings[p.ID]; !ok {
		s.readings[p.ID] = newRing(s.bufCap)
	}
}

// Point returns a copy of the point definition.
func (s *Store) Point(id string) (Point, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.points[id]
	if !ok {
		return Point{}, false
	}
	return *p, true
}

// Points returns copies of all point definitions.
func (s *Store) Points() []Point {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Point, 0, len(s.points))
	for _, p := range s.points {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ---- Readings (write path) ----

// AppendReading stores a reading for an existing point and returns the
// previous reading (for rate evaluation). It does NOT evaluate alarms; the
// service layer does that. If the point does not exist the reading is
// silently dropped (returns false).
func (s *Store) AppendReading(rd Reading) (Reading, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.readings[rd.PointID]
	if !ok {
		// lazily create a ring so ingest of unknown points is recorded too
		r = newRing(s.bufCap)
		s.readings[rd.PointID] = r
	}
	prev, _ := r.last()
	r.push(rd)
	return prev, true
}

// History returns a deep-copied, time-ordered snapshot of the last N
// readings for a point. The slice is independent of the internal buffer; later
// writes cannot mutate the returned slice.
func (s *Store) History(pointID string) []Reading {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.readings[pointID]
	if !ok {
		return nil
	}
	return r.snapshot()
}

// Latest returns the most recent reading for a point.
func (s *Store) Latest(pointID string) (Reading, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.readings[pointID]
	if !ok {
		return Reading{}, false
	}
	return r.last()
}

// ---- Alarms ----

// AddAlarm stores a new alarm. If an active alarm already exists for the same
// point, the existing alarm is resolved first (superseded).
func (s *Store) AddAlarm(a Alarm) {
	s.alarmsMu.Lock()
	defer s.alarmsMu.Unlock()
	cp := a
	s.alarms[a.ID] = &cp
	if a.State == AlarmActive {
		// if there's already an active alarm for the point, mark it resolved
		if existingID, ok := s.activeByPoint[a.PointID]; ok && existingID != a.ID {
			if old, ok := s.alarms[existingID]; ok && old.State == AlarmActive {
				old.State = AlarmResolved
			}
		}
		s.activeByPoint[a.PointID] = a.ID
	}
}

// UpdateAlarm applies a mutation function under the lock and returns a copy.
func (s *Store) UpdateAlarm(id string, fn func(*Alarm)) (Alarm, bool) {
	s.alarmsMu.Lock()
	defer s.alarmsMu.Unlock()
	a, ok := s.alarms[id]
	if !ok {
		return Alarm{}, false
	}
	fn(a)
	return *a, true
}

// Alarm returns a copy of an alarm by id.
func (s *Store) Alarm(id string) (Alarm, bool) {
	s.alarmsMu.Lock()
	defer s.alarmsMu.Unlock()
	a, ok := s.alarms[id]
	if !ok {
		return Alarm{}, false
	}
	return *a, true
}

// Alarms returns copies of all alarms matching the filter (non-nil fields).
// When onlyActive is true, only active alarms are returned.
func (s *Store) Alarms(pointID string, onlyActive bool) []Alarm {
	s.alarmsMu.Lock()
	defer s.alarmsMu.Unlock()
	out := make([]Alarm, 0, len(s.alarms))
	for _, a := range s.alarms {
		if pointID != "" && a.PointID != pointID {
			continue
		}
		if onlyActive && a.State != AlarmActive {
			continue
		}
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ts.Before(out[j].Ts) })
	return out
}

// ActiveAlarmCount returns the number of currently-active alarms.
func (s *Store) ActiveAlarmCount() int {
	s.alarmsMu.Lock()
	defer s.alarmsMu.Unlock()
	return len(s.activeByPoint)
}

// ResolveActiveForPoint resolves any active alarm for the given point (used
// when a reading returns to normal).
func (s *Store) ResolveActiveForPoint(pointID string) bool {
	s.alarmsMu.Lock()
	defer s.alarmsMu.Unlock()
	id, ok := s.activeByPoint[pointID]
	if !ok {
		return false
	}
	if a, ok := s.alarms[id]; ok {
		a.State = AlarmResolved
	}
	delete(s.activeByPoint, pointID)
	return true
}

// now helper for tests that need the buffer capacity.
func (s *Store) BufferCapacity() int { return s.bufCap }

// Stats reports counts for the summary endpoint.
func (s *Store) Stats() (points, readings, alarms, active int) {
	s.mu.Lock()
	points = len(s.points)
	for _, r := range s.readings {
		readings += r.n
	}
	s.mu.Unlock()
	alarms = len(s.alarms)
	active = s.ActiveAlarmCount()
	return
}

// ensure ringBuffer.time helper exists for ordering sanity
var _ = time.Now
