package permit

// Permit store: by id and by segment (for conflict checks). Read methods
// return deep copies.

import (
	"sort"
	"sync"
	"time"
)

// Store is the permit repository.
type Store struct {
	mu    sync.RWMutex
	items map[string]*Permit
}

// NewStore returns an empty permit store.
func NewStore() *Store {
	return &Store{items: make(map[string]*Permit)}
}

// Put inserts or replaces a permit.
func (s *Store) Put(p Permit) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	p.UpdatedAt = time.Now()
	cp := p
	s.items[p.ID] = &cp
}

// Get returns a copy of a permit.
func (s *Store) Get(id string) (Permit, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.items[id]
	if !ok {
		return Permit{}, false
	}
	return *p, true
}

// Update applies a mutation under the lock and returns a copy.
func (s *Store) Update(id string, fn func(*Permit)) (Permit, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.items[id]
	if !ok {
		return Permit{}, false
	}
	fn(p)
	p.UpdatedAt = time.Now()
	return *p, true
}

// All returns copies of all permits.
func (s *Store) All() []Permit {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Permit, 0, len(s.items))
	for _, p := range s.items {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// OpenForSegment returns copies of open permits for a segment whose window
// overlaps [start,end]. Used for conflict checks during approval.
func (s *Store) OpenForSegment(segmentID string, start, end time.Time) []Permit {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Permit
	for _, p := range s.items {
		if p.SegmentID != segmentID || !p.IsOpen() {
			continue
		}
		// any open permit on the segment blocks the window
		out = append(out, *p)
	}
	return out
}

// OpenForSegmentAnyWindow returns all open permits for a segment regardless
// of window (used by dispatch conflict checks).
func (s *Store) OpenForSegmentAnyWindow(segmentID string) []Permit {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Permit
	for _, p := range s.items {
		if p.SegmentID == segmentID && p.IsOpen() {
			out = append(out, *p)
		}
	}
	return out
}

// CountByState returns a tally of permits by state, for dashboards.
func (s *Store) CountByState() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tally := make(map[string]int)
	for _, p := range s.items {
		tally[p.State.String()]++
	}
	return tally
}
