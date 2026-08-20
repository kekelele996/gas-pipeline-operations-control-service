package contract

// Contract store: keeps contracts by id and shipper. Capacity operations are
// guarded by a per-contract mutex so concurrent nominations on the same
// contract cannot oversell. Read methods return deep copies.

import (
	"sort"
	"sync"
	"time"
)

// Store is the contract repository.
type Store struct {
	mu        sync.RWMutex
	contracts map[string]*Contract
	byShipper map[string][]string
	locks     sync.Map // contract id -> *sync.Mutex
}

// NewStore returns an empty contract store.
func NewStore() *Store {
	return &Store{
		contracts: make(map[string]*Contract),
		byShipper: make(map[string][]string),
	}
}

// Put inserts or replaces a contract.
func (s *Store) Put(c Contract) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	c.UpdatedAt = time.Now()
	cp := c
	s.contracts[c.ID] = &cp
	s.byShipper[c.ShipperID] = appendUnique(s.byShipper[c.ShipperID], c.ID)
}

// Get returns a copy of a contract.
func (s *Store) Get(id string) (Contract, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.contracts[id]
	if !ok {
		return Contract{}, false
	}
	return *c, true
}

// All returns copies of all contracts.
func (s *Store) All() []Contract {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Contract, 0, len(s.contracts))
	for _, c := range s.contracts {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ByShipper returns copies of a shipper's contracts.
func (s *Store) ByShipper(shipperID string) []Contract {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byShipper[shipperID]
	out := make([]Contract, 0, len(ids))
	for _, id := range ids {
		if c, ok := s.contracts[id]; ok {
			out = append(out, *c)
		}
	}
	return out
}

// Update applies a mutation function under the per-contract lock and returns a
// copy. The per-contract mutex serializes capacity mutations so concurrent
// nominations cannot oversell.
func (s *Store) Update(id string, fn func(*Contract)) (Contract, bool) {
	mu := s.lockFor(id)
	mu.Lock()
	defer mu.Unlock()

	s.mu.Lock()
	c, ok := s.contracts[id]
	if !ok {
		s.mu.Unlock()
		return Contract{}, false
	}
	fn(c)
	c.UpdatedAt = time.Now()
	out := *c
	s.mu.Unlock()
	return out, true
}

// lockFor returns the per-contract mutex, creating it on first use.
func (s *Store) lockFor(id string) *sync.Mutex {
	v, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}
