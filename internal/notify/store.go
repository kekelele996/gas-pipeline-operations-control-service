package notify

// Notification store: by id, with queued/failed indices for batch operations.
// Read methods return deep copies.

import (
	"sort"
	"sync"
	"time"
)

// Store is the notification repository.
type Store struct {
	mu    sync.RWMutex
	items map[string]*Notification
}

// NewStore returns an empty notification store.
func NewStore() *Store {
	return &Store{items: make(map[string]*Notification)}
}

// Put inserts or replaces a notification.
func (s *Store) Put(n Notification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	n.UpdatedAt = time.Now()
	cp := n
	s.items[n.ID] = &cp
}

// Get returns a copy of a notification.
func (s *Store) Get(id string) (Notification, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.items[id]
	if !ok {
		return Notification{}, false
	}
	return *n, true
}

// Update applies a mutation under the lock and returns a copy.
func (s *Store) Update(id string, fn func(*Notification)) (Notification, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.items[id]
	if !ok {
		return Notification{}, false
	}
	fn(n)
	n.UpdatedAt = time.Now()
	return *n, true
}

// All returns copies of all notifications.
func (s *Store) All() []Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Notification, 0, len(s.items))
	for _, n := range s.items {
		out = append(out, *n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Queued returns copies of notifications in queued or retrying state (due for
// a push attempt).
func (s *Store) Due(now time.Time) []Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Notification
	for _, n := range s.items {
		if n.State != StateQueued && n.State != StateRetrying {
			continue
		}
		// retrying messages are treated as due immediately; the caller
		// (PushBatch) applies backoff when it marks them again
		out = append(out, *n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

// CountByState returns a tally for dashboards.
func (s *Store) CountByState() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tally := make(map[string]int)
	for _, n := range s.items {
		tally[n.State.String()]++
	}
	return tally
}
