package audit

import (
	"sort"
	"sync"
	"time"
)

// Store is an append-only in-memory audit log. When the number of entries
// exceeds retention, the oldest entries are dropped. All methods are safe for
// concurrent use; Query returns deep copies.
type Store struct {
	mu        sync.RWMutex
	entries   []Entry
	retention int
}

// NewStore returns an audit store with the given retention limit. A retention
// of 0 or less means unlimited.
func NewStore(retention int) *Store {
	if retention < 0 {
		retention = 0
	}
	return &Store{retention: retention}
}

// Append records an entry. The entry's ID is set if empty.
func (s *Store) Append(e Entry) {
	if e.ID == "" {
		e.ID = "AUD-" + e.Ts.Format("20060102150405") + "-" + randSuffix()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.retention > 0 && len(s.entries) >= s.retention {
		// drop the oldest 10% to amortize shifting cost
		drop := len(s.entries) / 10
		if drop < 1 {
			drop = 1
		}
		s.entries = s.entries[drop:]
	}
	s.entries = append(s.entries, e)
}

// Get returns a copy of a single entry by id.
func (s *Store) Get(id string) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// All returns copies of all entries (newest last).
func (s *Store) All() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	return out
}

// Query returns entries matching the filter, newest-first, limited.
func (s *Store) Query(q Query) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Entry
	for _, e := range s.entries {
		if !match(q, e) {
			continue
		}
		out = append(out, e)
	}
	// newest first
	sort.Slice(out, func(i, j int) bool { return out[i].Ts.After(out[j].Ts) })
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out
}

// Count returns the number of stored entries.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

func match(q Query, e Entry) bool {
	if q.Actor != "" && e.Actor != q.Actor {
		return false
	}
	if q.Action != "" && e.Action != q.Action {
		return false
	}
	if q.TargetType != "" && e.TargetType != q.TargetType {
		return false
	}
	if q.TargetID != "" && e.TargetID != q.TargetID {
		return false
	}
	if !q.Since.IsZero() && e.Ts.Before(q.Since) {
		return false
	}
	if !q.Until.IsZero() && e.Ts.After(q.Until) {
		return false
	}
	return true
}

// randSuffix produces a short hex suffix without importing crypto at package
// init (keeps the store cheap). It uses time-based entropy; uniqueness within
// the process is adequate for an audit log.
func randSuffix() string {
	t := time.Now().UnixNano()
	const digits = "0123456789abcdef"
	var b [6]byte
	for i := 5; i >= 0; i-- {
		b[i] = digits[t&0xf]
		t >>= 4
	}
	return string(b[:])
}
