package nomination

// Nomination store: by id and by (contract,date). Read methods return deep
// copies and are safe for concurrent use.

import (
	"sort"
	"sync"
	"time"
)

// Store is the nomination repository.
type Store struct {
	mu             sync.RWMutex
	items          map[string]*Nomination
	byContractDate map[dateKey][]string
}

type dateKey struct {
	contract string
	date     string
}

// NewStore returns an empty nomination store.
func NewStore() *Store {
	return &Store{
		items:          make(map[string]*Nomination),
		byContractDate: make(map[dateKey][]string),
	}
}

// Put inserts or replaces a nomination.
func (s *Store) Put(n Nomination) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	n.UpdatedAt = time.Now()
	cp := n
	s.items[n.ID] = &cp
	key := dateKey{n.ContractID, n.Date}
	s.byContractDate[key] = appendUnique(s.byContractDate[key], n.ID)
}

// Get returns a copy of a nomination.
func (s *Store) Get(id string) (Nomination, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.items[id]
	if !ok {
		return Nomination{}, false
	}
	return *n, true
}

// Update applies a mutation under the lock and returns a copy.
func (s *Store) Update(id string, fn func(*Nomination)) (Nomination, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.items[id]
	if !ok {
		return Nomination{}, false
	}
	fn(n)
	n.UpdatedAt = time.Now()
	return *n, true
}

// All returns copies of all nominations.
func (s *Store) All() []Nomination {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Nomination, 0, len(s.items))
	for _, n := range s.items {
		out = append(out, *n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ForContractDate returns copies of nominations for a contract on a date.
func (s *Store) ForContractDate(contractID, date string) []Nomination {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byContractDate[dateKey{contractID, date}]
	out := make([]Nomination, 0, len(ids))
	for _, id := range ids {
		if n, ok := s.items[id]; ok {
			out = append(out, *n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}
