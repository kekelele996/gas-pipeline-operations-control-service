package audit

import (
	"context"
	"fmt"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
)

// Service records and queries audit entries. It satisfies the network
// AuditRecorder and dispatch recorder interfaces.
type Service struct {
	store *Store
	clock platform.Clock
}

// NewService builds an audit service over the given store. If clock is nil a
// system clock is used.
func NewService(store *Store, clock platform.Clock) *Service {
	if clock == nil {
		clock = platform.SystemClock{}
	}
	return &Service{store: store, clock: clock}
}

// Store exposes the underlying store for read-only access.
func (s *Service) Store() *Store { return s.store }

// Record appends an audit entry. The timestamp is set from the service clock
// unless already populated. It never returns an error (logging must not
// break the calling operation) — failures are swallowed to keep audit a
// best-effort side-channel.
func (s *Service) Record(ctx context.Context, actor, action, targetType, targetID, detail string) error {
	ts := s.clock.Now()
	e := Entry{
		Actor: actor, Action: action, TargetType: targetType,
		TargetID: targetID, Detail: detail, Result: "ok", Ts: ts,
	}
	s.store.Append(e)
	return nil
}

// RecordResult records an entry with an explicit result string (e.g. an error
// outcome), so operators can see when an action failed.
func (s *Service) RecordResult(ctx context.Context, actor, action, targetType, targetID, detail, result string) error {
	e := Entry{
		Actor: actor, Action: action, TargetType: targetType,
		TargetID: targetID, Detail: detail, Result: result, Ts: s.clock.Now(),
	}
	s.store.Append(e)
	return nil
}

// Get retrieves a single entry by id.
func (s *Service) Get(ctx context.Context, id string) (Entry, error) {
	e, ok := s.store.Get(id)
	if !ok {
		return Entry{}, platform.NotFoundf("audit entry %q not found", id)
	}
	return e, nil
}

// Query returns entries matching the filter.
func (s *Service) Query(ctx context.Context, q Query) []Entry {
	if q.Limit == 0 {
		q.Limit = 100
	}
	return s.store.Query(q)
}

// Count returns the number of stored entries.
func (s *Service) Count(ctx context.Context) int {
	return s.store.Count()
}

// Summary returns a compact per-action tally, useful for dashboards.
func (s *Service) Summary(ctx context.Context) Summary {
	entries := s.store.All()
	tally := make(map[string]int, 32)
	for _, e := range entries {
		tally[e.Action]++
	}
	return Summary{Total: len(entries), ByAction: tally}
}

// Summary is the aggregate audit view.
type Summary struct {
	Total    int            `json:"total"`
	ByAction map[string]int `json:"by_action"`
}

// FormatEntry renders an entry as a single log-style line.
func FormatEntry(e Entry) string {
	result := e.Result
	if result == "" {
		result = "ok"
	}
	return fmt.Sprintf("%s [%s] %s %s/%s %s (%s)",
		e.Ts.Format(time.RFC3339), e.Actor, e.Action, e.TargetType, e.TargetID, e.Detail, result)
}

// Recent returns the newest n entries. The returned slice may share storage
// with the log; callers must not mutate it.
func (s *Service) Recent(ctx context.Context, n int) []Entry {
	all := s.store.entries
	if n <= 0 || len(all) == 0 {
		return nil
	}
	if len(all) > n {
		all = all[len(all)-n:]
	}
	return all
}
