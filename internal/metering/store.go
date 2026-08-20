package metering

// Metering store: meters, daily totals keyed by (meter,date), and settlements
// keyed by date. All methods return deep copies and are safe for concurrent
// use.

import (
	"sort"
	"sync"
	"time"
)

// Store is the metering in-memory repository.
type Store struct {
	mu sync.RWMutex

	meters      map[string]*Meter
	daily       map[dailyKey]*DailyTotal // keyed by (meter,date)
	settlements map[string]*Settlement   // keyed by date

	// rolling raw readings (for settlement rebuild); keep last N per meter
	readingsMu sync.Mutex
	readings   map[string][]RawReading
	readingCap int
}

type dailyKey struct {
	meter string
	date  string
}

// NewStore returns an empty metering store. readingCap bounds how many recent
// raw readings are retained per meter for settlement rebuilding.
func NewStore(readingCap int) *Store {
	if readingCap < 1 {
		readingCap = 5000
	}
	return &Store{
		meters:      make(map[string]*Meter),
		daily:       make(map[dailyKey]*DailyTotal),
		settlements: make(map[string]*Settlement),
		readings:    make(map[string][]RawReading),
		readingCap:  readingCap,
	}
}

// ---- Meters ----

// PutMeter inserts or replaces a meter definition. Factors are sorted by
// TempLo so band lookup is a linear scan.
func (s *Store) PutMeter(m Meter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	m.UpdatedAt = time.Now()
	sort.Slice(m.Factors, func(i, j int) bool { return m.Factors[i].TempLo < m.Factors[j].TempLo })
	cp := m
	s.meters[m.ID] = &cp
}

// Meter returns a copy of the meter definition.
func (s *Store) Meter(id string) (Meter, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.meters[id]
	if !ok {
		return Meter{}, false
	}
	return *m, true
}

// Meters returns copies of all meters.
func (s *Store) Meters() []Meter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Meter, 0, len(s.meters))
	for _, m := range s.meters {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ---- Daily totals ----

// UpsertDaily applies an updater to the daily total for (meter,date). The
// updater receives a pointer to the (possibly newly created) total. Returns a
// copy of the result.
func (s *Store) UpsertDaily(meterID, date string, fn func(*DailyTotal)) DailyTotal {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := dailyKey{meterID, date}
	t, ok := s.daily[key]
	if !ok {
		t = &DailyTotal{MeterID: meterID, Date: date}
		s.daily[key] = t
	}
	fn(t)
	return *t
}

// DailyTotal returns a copy of the daily total for (meter,date).
func (s *Store) DailyTotal(meterID, date string) (DailyTotal, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.daily[dailyKey{meterID, date}]
	if !ok {
		return DailyTotal{}, false
	}
	return *t, true
}

// AllDailyForDate returns copies of every meter's daily total for a date.
func (s *Store) AllDailyForDate(date string) []DailyTotal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []DailyTotal
	for k, t := range s.daily {
		if k.date == date {
			out = append(out, *t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MeterID < out[j].MeterID })
	return out
}

// ---- Raw readings (retained for settlement rebuild) ----

// AppendReading retains a raw reading for later settlement rebuild.
func (s *Store) AppendReading(rd RawReading) {
	s.readingsMu.Lock()
	defer s.readingsMu.Unlock()
	rs := s.readings[rd.MeterID]
	rs = append(rs, rd)
	if len(rs) > s.readingCap {
		rs = rs[len(rs)-s.readingCap:]
	}
	s.readings[rd.MeterID] = rs
}

// ReadingsFor returns copies of the retained raw readings for a meter.
func (s *Store) ReadingsFor(meterID string) []RawReading {
	s.readingsMu.Lock()
	defer s.readingsMu.Unlock()
	src := s.readings[meterID]
	out := make([]RawReading, len(src))
	copy(out, src)
	return out
}

// ---- Settlements ----

// PutSettlement inserts or replaces a settlement keyed by its Date.
func (s *Store) PutSettlement(st Settlement) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st.CreatedAt.IsZero() {
		st.CreatedAt = time.Now()
	}
	st.UpdatedAt = time.Now()
	cp := st
	s.settlements[st.Date] = &cp
}

// Settlement returns a copy of the settlement for a date.
func (s *Store) Settlement(date string) (Settlement, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.settlements[date]
	if !ok {
		return Settlement{}, false
	}
	return *st, true
}

// UpdateSettlement applies a mutation function under the lock and returns a copy.
func (s *Store) UpdateSettlement(date string, fn func(*Settlement)) (Settlement, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.settlements[date]
	if !ok {
		return Settlement{}, false
	}
	fn(st)
	st.UpdatedAt = time.Now()
	return *st, true
}

// Settlements returns copies of all settlements.
func (s *Store) Settlements() []Settlement {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Settlement, 0, len(s.settlements))
	for _, st := range s.settlements {
		out = append(out, *st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

// dateOf formats a time as the canonical date string (local calendar day).
func dateOf(t time.Time) string {
	return t.Format("2006-01-02")
}
