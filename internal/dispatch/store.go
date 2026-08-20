package dispatch

// Dispatch store: by id and by segment. Read methods return deep copies.

import (
	"sort"
	"sync"
	"time"
)

// Store is the dispatch order repository.
type Store struct {
	mu    sync.RWMutex
	items map[string]*Order
}

// NewStore returns an empty dispatch store.
func NewStore() *Store {
	return &Store{items: make(map[string]*Order)}
}

// Put inserts or replaces an order.
func (s *Store) Put(o Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now()
	}
	o.UpdatedAt = time.Now()
	cp := o
	s.items[o.ID] = &cp
}

// Get returns a copy of an order.
func (s *Store) Get(id string) (Order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.items[id]
	if !ok {
		return Order{}, false
	}
	return *o, true
}

// Update applies a mutation under the lock and returns a copy.
func (s *Store) Update(id string, fn func(*Order)) (Order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.items[id]
	if !ok {
		return Order{}, false
	}
	fn(o)
	o.UpdatedAt = time.Now()
	return *o, true
}

// All returns copies of all orders.
func (s *Store) All() []Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Order, 0, len(s.items))
	for _, o := range s.items {
		out = append(out, *o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// PendingForSegment returns copies of non-terminal orders for a segment.
func (s *Store) PendingForSegment(segmentID string) []Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Order
	for _, o := range s.items {
		if o.SegmentID == segmentID && (o.State == StatePending || o.State == StateIssued) {
			out = append(out, *o)
		}
	}
	return out
}

// CountByState returns a tally for dashboards.
func (s *Store) CountByState() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tally := make(map[string]int)
	for _, o := range s.items {
		tally[o.State.String()]++
	}
	return tally
}
