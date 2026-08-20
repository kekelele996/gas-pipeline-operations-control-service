package incident

// Incident store: by id and by segment. Read methods return deep copies.

import (
	"sort"
	"sync"
	"time"
)

// Store is the incident repository.
type Store struct {
	mu    sync.RWMutex
	items map[string]*Incident
}

// NewStore returns an empty incident store.
func NewStore() *Store {
	return &Store{items: make(map[string]*Incident)}
}

// Put inserts or replaces an incident.
func (s *Store) Put(i Incident) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i.CreatedAt.IsZero() {
		i.CreatedAt = time.Now()
	}
	i.UpdatedAt = time.Now()
	cp := i
	s.items[i.ID] = &cp
}

// Get returns a copy of an incident.
func (s *Store) Get(id string) (Incident, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, ok := s.items[id]
	if !ok {
		return Incident{}, false
	}
	return *i, true
}

// Update applies a mutation under the lock and returns a copy.
func (s *Store) Update(id string, fn func(*Incident)) (Incident, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.items[id]
	if !ok {
		return Incident{}, false
	}
	fn(i)
	i.UpdatedAt = time.Now()
	return *i, true
}

// All returns copies of all incidents.
func (s *Store) All() []Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Incident, 0, len(s.items))
	for _, i := range s.items {
		out = append(out, *i)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// OpenForSegment returns copies of open (non-closed) incidents for a segment.
func (s *Store) OpenForSegment(segmentID string) []Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Incident
	for _, i := range s.items {
		if i.SegmentID == segmentID && i.State != StateClosed {
			out = append(out, *i)
		}
	}
	return out
}

// OpenCount returns the number of open incidents.
func (s *Store) OpenCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, i := range s.items {
		if i.State != StateClosed {
			n++
		}
	}
	return n
}
