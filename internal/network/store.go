package network

import (
	"sort"
	"sync"
	"time"
)

// Store is the in-memory topology repository. It indexes by id and by
// segment/station so common adjacency queries are O(1). All read methods
// return deep copies so callers can never mutate internal state.
type Store struct {
	mu sync.RWMutex

	segments    map[string]*Segment
	stations    map[string]*Station
	compressors map[string]*Compressor
	valves      map[string]*Valve
	points      map[string]*Point

	// index: station -> connected segments (derived, not authoritative)
	stationSegments map[string][]string
	// index: segment -> devices on it
	segmentDevices map[string]DeviceRefs

	// limits holds operator-set operating limits per segment / station.
	limits map[string]float64
}

// DeviceRefs groups device ids attached to a segment.
type DeviceRefs struct {
	Compressors []string
	Valves      []string
	Points      []string
}

// NewStore returns an empty topology store.
func NewStore() *Store {
	return &Store{
		segments:        make(map[string]*Segment),
		stations:        make(map[string]*Station),
		compressors:     make(map[string]*Compressor),
		valves:          make(map[string]*Valve),
		points:          make(map[string]*Point),
		stationSegments: make(map[string][]string),
		segmentDevices:  make(map[string]DeviceRefs),
	}
}

// ---- Segments ----

// PutSegment inserts or replaces a segment, updating station adjacency.
func (s *Store) PutSegment(seg Segment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if seg.CreatedAt.IsZero() {
		seg.CreatedAt = time.Now()
	}
	seg.UpdatedAt = time.Now()
	cp := seg
	s.segments[seg.ID] = &cp
	s.reindexSegment(&cp)
}

// Segment returns a copy of the segment with the given id.
func (s *Store) Segment(id string) (Segment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seg, ok := s.segments[id]
	if !ok {
		return Segment{}, false
	}
	return *seg, true
}

// DeleteSegment removes a segment and clears its adjacency entries.
func (s *Store) DeleteSegment(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	seg, ok := s.segments[id]
	if !ok {
		return false
	}
	delete(s.segments, id)
	// drop adjacency for the segment from its endpoint stations
	s.removeStationSegment(seg.From, id)
	s.removeStationSegment(seg.To, id)
	delete(s.segmentDevices, id)
	return true
}

// Segments returns copies of all segments, sorted by id for stable output.
func (s *Store) Segments() []Segment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Segment, 0, len(s.segments))
	for _, seg := range s.segments {
		out = append(out, *seg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ---- Stations ----

// PutStation inserts or replaces a station.
func (s *Store) PutStation(st Station) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st.CreatedAt.IsZero() {
		st.CreatedAt = time.Now()
	}
	st.UpdatedAt = time.Now()
	cp := st
	s.stations[st.ID] = &cp
}

// Station returns a copy of the station with the given id.
func (s *Store) Station(id string) (Station, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.stations[id]
	if !ok {
		return Station{}, false
	}
	return *st, true
}

// Stations returns copies of all stations, sorted by id.
func (s *Store) Stations() []Station {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Station, 0, len(s.stations))
	for _, st := range s.stations {
		out = append(out, *st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// StationSegments returns the ids of segments connected to the given station.
func (s *Store) StationSegments(stationID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.stationSegments[stationID]...)
}

// ---- Compressors ----

// PutCompressor inserts or replaces a compressor, indexing it by segment.
func (s *Store) PutCompressor(c Compressor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	c.UpdatedAt = time.Now()
	cp := c
	s.compressors[c.ID] = &cp
	d := s.segmentDevices[c.SegmentID]
	d.Compressors = appendUnique(d.Compressors, c.ID)
	s.segmentDevices[c.SegmentID] = d
}

// Compressor returns a copy of the compressor with the given id.
func (s *Store) Compressor(id string) (Compressor, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.compressors[id]
	if !ok {
		return Compressor{}, false
	}
	return *c, true
}

// UpdateCompressor applies a mutation function under the store lock and
// returns a copy of the result. The function must not retain the pointer.
func (s *Store) UpdateCompressor(id string, fn func(*Compressor)) (Compressor, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.compressors[id]
	if !ok {
		return Compressor{}, false
	}
	fn(c)
	c.UpdatedAt = time.Now()
	return *c, true
}

// Compressors returns copies of all compressors.
func (s *Store) Compressors() []Compressor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Compressor, 0, len(s.compressors))
	for _, c := range s.compressors {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// SegmentCompressors returns copies of compressors on the given segment.
func (s *Store) SegmentCompressors(segmentID string) []Compressor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	refs := s.segmentDevices[segmentID].Compressors
	out := make([]Compressor, 0, len(refs))
	for _, id := range refs {
		if c, ok := s.compressors[id]; ok {
			out = append(out, *c)
		}
	}
	return out
}

// ---- Valves ----

// PutValve inserts or replaces a valve, indexing it by segment.
func (s *Store) PutValve(v Valve) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	v.UpdatedAt = time.Now()
	cp := v
	s.valves[v.ID] = &cp
	d := s.segmentDevices[v.SegmentID]
	d.Valves = appendUnique(d.Valves, v.ID)
	s.segmentDevices[v.SegmentID] = d
}

// Valve returns a copy of the valve with the given id.
func (s *Store) Valve(id string) (Valve, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.valves[id]
	if !ok {
		return Valve{}, false
	}
	return *v, true
}

// UpdateValve applies a mutation function under the store lock and returns a copy.
func (s *Store) UpdateValve(id string, fn func(*Valve)) (Valve, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.valves[id]
	if !ok {
		return Valve{}, false
	}
	fn(v)
	v.UpdatedAt = time.Now()
	return *v, true
}

// Valves returns copies of all valves.
func (s *Store) Valves() []Valve {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Valve, 0, len(s.valves))
	for _, v := range s.valves {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// SegmentValves returns copies of valves on the given segment.
func (s *Store) SegmentValves(segmentID string) []Valve {
	s.mu.RLock()
	defer s.mu.RUnlock()
	refs := s.segmentDevices[segmentID].Valves
	out := make([]Valve, 0, len(refs))
	for _, id := range refs {
		if v, ok := s.valves[id]; ok {
			out = append(out, *v)
		}
	}
	return out
}

// ---- Points ----

// PutPoint inserts or replaces a measurement point, indexing it by segment.
func (s *Store) PutPoint(p Point) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	p.UpdatedAt = time.Now()
	cp := p
	s.points[p.ID] = &cp
	if p.SegmentID != "" {
		d := s.segmentDevices[p.SegmentID]
		d.Points = appendUnique(d.Points, p.ID)
		s.segmentDevices[p.SegmentID] = d
	}
}

// Point returns a copy of the point with the given id.
func (s *Store) Point(id string) (Point, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.points[id]
	if !ok {
		return Point{}, false
	}
	return *p, true
}

// Points returns copies of all points.
func (s *Store) Points() []Point {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Point, 0, len(s.points))
	for _, p := range s.points {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// SegmentPoints returns copies of points on the given segment.
func (s *Store) SegmentPoints(segmentID string) []Point {
	s.mu.RLock()
	defer s.mu.RUnlock()
	refs := s.segmentDevices[segmentID].Points
	out := make([]Point, 0, len(refs))
	for _, id := range refs {
		if p, ok := s.points[id]; ok {
			out = append(out, *p)
		}
	}
	return out
}

// UpdatePoint applies a mutation function under the store lock and returns a copy.
func (s *Store) UpdatePoint(id string, fn func(*Point)) (Point, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.points[id]
	if !ok {
		return Point{}, false
	}
	fn(p)
	p.UpdatedAt = time.Now()
	return *p, true
}

// ---- index helpers (must be called with write lock held) ----

func (s *Store) reindexSegment(seg *Segment) {
	s.addStationSegment(seg.From, seg.ID)
	s.addStationSegment(seg.To, seg.ID)
}

func (s *Store) addStationSegment(stationID, segmentID string) {
	if stationID == "" {
		return
	}
	for _, existing := range s.stationSegments[stationID] {
		if existing == segmentID {
			return
		}
	}
	s.stationSegments[stationID] = append(s.stationSegments[stationID], segmentID)
}

func (s *Store) removeStationSegment(stationID, segmentID string) {
	ids := s.stationSegments[stationID]
	for i, id := range ids {
		if id == segmentID {
			s.stationSegments[stationID] = append(ids[:i], ids[i+1:]...)
			if len(s.stationSegments[stationID]) == 0 {
				delete(s.stationSegments, stationID)
			}
			return
		}
	}
}

func appendUnique(slice []string, v string) []string {
	for _, s := range slice {
		if s == v {
			return slice
		}
	}
	return append(slice, v)
}

// Counts returns aggregate counts for the summary endpoint.
func (s *Store) Counts() (segments, stations, compressors, valves, points int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.segments), len(s.stations), len(s.compressors), len(s.valves), len(s.points)
}

// ---- Operating limits ----

// PutSegmentLimit stores an operator-set limit for a segment.
func (s *Store) PutSegmentLimit(segmentID string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.limits[segmentID] = value
}

// PutStationLimit stores an operator-set limit for a station.
func (s *Store) PutStationLimit(stationID string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.limits[stationID] = value
}

// SegmentLimit returns the operator-set limit for a segment, if any.
func (s *Store) SegmentLimit(segmentID string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.limits[segmentID]
	return v, ok
}

// PutValveLimit stores an operator-set limit for a valve.
func (s *Store) PutValveLimit(valveID string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.limits[valveID] = value
}

// PutCompressorLimit stores an operator-set limit for a compressor.
func (s *Store) PutCompressorLimit(compressorID string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.limits[compressorID] = value
}

// ValveLimit returns the operator-set limit for a valve, if any.
func (s *Store) ValveLimit(valveID string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.limits[valveID]
	return v, ok
}

// CompressorLimit returns the operator-set limit for a compressor, if any.
func (s *Store) CompressorLimit(compressorID string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.limits[compressorID]
	return v, ok
}
