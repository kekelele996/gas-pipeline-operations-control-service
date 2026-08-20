package permit

// Permit service: apply for, approve (with conflict checks), start, complete,
// cancel, and expire-scan permits. Approval is denied when another open
// permit, a pending dispatch order, or an open incident overlaps the segment.

import (
	"context"
	"fmt"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
)

// AuditRecorder mirrors the network package's recorder interface.
type AuditRecorder interface {
	Record(ctx context.Context, actor, action, targetType, targetID, detail string) error
}

// Clock is the time source used for expiry scanning.
type Clock = platform.Clock

// Service implements permit business logic.
type Service struct {
	store    *Store
	clock    platform.Clock
	dispatch DispatchConflictChecker
	incident IncidentConflictChecker
	audit    AuditRecorder
}

// NewService builds a permit service. dispatch and incident may be nil; in that
// case the corresponding conflict check is skipped.
func NewService(store *Store, clock platform.Clock, dispatch DispatchConflictChecker, incident IncidentConflictChecker, audit AuditRecorder) *Service {
	if clock == nil {
		clock = platform.SystemClock{}
	}
	if dispatch == nil {
		dispatch = noopDispatchChecker{}
	}
	if incident == nil {
		incident = noopIncidentChecker{}
	}
	return &Service{store: store, clock: clock, dispatch: dispatch, incident: incident, audit: audit}
}

// Store exposes the underlying store (read-only consumers).
func (s *Service) Store() *Store { return s.store }

// Input is the payload for applying for a permit.
type Input struct {
	SegmentID   string    `json:"segment_id"`
	StationID   string    `json:"station_id"`
	WorkType    string    `json:"work_type"`
	Title       string    `json:"title"`
	Applicant   string    `json:"applicant"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	Reason      string    `json:"reason"`
}

// Apply validates and stores a new permit in the pending state (skip draft for
// a submitted application). The window must be valid and not too far in the
// future (controlled by the caller — we only enforce ordering here).
func (s *Service) Apply(ctx context.Context, in Input) (Permit, error) {
	v := platform.NewValidate()
	v.RequireNonEmpty("segment_id", in.SegmentID)
	v.RequireNonEmpty("work_type", in.WorkType)
	v.RequireNonEmpty("title", in.Title)
	v.RequireNonEmpty("applicant", in.Applicant)
	v.RequireEnum("work_type", in.WorkType, []string{"pigging", "welding", "inspection", "repair", "excavation"})
	if !in.WindowEnd.After(in.WindowStart) {
		v.Require(false, "window_end must be after window_start")
	}
	if err := v.Error(); err != nil {
		return Permit{}, err
	}
	p := Permit{
		ID: platform.NewPermitID(), SegmentID: in.SegmentID, StationID: in.StationID,
		WorkType: in.WorkType, Title: in.Title, Applicant: in.Applicant,
		WindowStart: in.WindowStart, WindowEnd: in.WindowEnd,
		Reason: in.Reason, State: StatePending,
	}
	s.store.Put(p)
	out, _ := s.store.Get(p.ID)
	if s.audit != nil {
		_ = s.audit.Record(ctx, in.Applicant, "apply_permit", "permit", p.ID,
			fmt.Sprintf("%s on %s %s..%s", in.WorkType, in.SegmentID,
				in.WindowStart.Format(time.RFC3339), in.WindowEnd.Format(time.RFC3339)))
	}
	return out, nil
}

// Approve moves a pending permit to approved, after verifying no conflicts
// with other open permits, pending dispatch orders, or open incidents on the
// same segment.
func (s *Service) Approve(ctx context.Context, id, approver string) (p Permit, err error) {
	p, ok := s.store.Get(id)
	if !ok {
		return Permit{}, platform.NotFoundf("permit %q not found", id)
	}
	defer func() {
		// Audit is a best-effort side-channel; it must never clobber the
		// business error returned to the caller.
		if s.audit != nil {
			_ = s.audit.Record(ctx, approver, "approve_permit", "permit", id, "")
		}
	}()
	next, err := MustTransition(p.State, StateApproved)
	if err != nil {
		return p, err
	}
	// conflict checks
	if conflicts := s.store.OpenForSegment(p.SegmentID, p.WindowStart, p.WindowEnd); len(conflicts) > 0 {
		// exclude self
		others := make([]string, 0, len(conflicts))
		for _, c := range conflicts {
			if c.ID != id {
				others = append(others, c.ID)
			}
		}
		if len(others) > 0 {
			return p, platform.Conflictf("overlapping permits on %s: %v", p.SegmentID, others)
		}
	}
	if ids := s.dispatch.PendingOrdersForSegment(p.SegmentID); len(ids) > 0 {
		return p, platform.Conflictf("pending dispatch orders on %s: %v", p.SegmentID, ids)
	}
	if ids := s.incident.OpenIncidentsForSegment(p.SegmentID); len(ids) > 0 {
		return p, platform.Conflictf("open incidents on %s: %v", p.SegmentID, ids)
	}
	out, _ := s.store.Update(id, func(x *Permit) {
		x.State = next
		x.Approver = approver
	})
	return out, nil
}

// Start moves an approved permit to in_progress.
func (s *Service) Start(ctx context.Context, id string) (p Permit, err error) {
	p, ok := s.store.Get(id)
	if !ok {
		return Permit{}, platform.NotFoundf("permit %q not found", id)
	}
	defer func() {
		// Audit is best-effort; never overwrite the returned business error.
		if s.audit != nil {
			_ = s.audit.Record(ctx, "operator", "start_permit", "permit", id, "")
		}
	}()
	next, err := MustTransition(p.State, StateInProgress)
	if err != nil {
		return p, err
	}
	out, _ := s.store.Update(id, func(x *Permit) {
		x.State = next
		x.StartedAt = s.clock.Now()
	})
	return out, nil
}

// Complete moves an in_progress permit to completed.
func (s *Service) Complete(ctx context.Context, id string) (p Permit, err error) {
	p, ok := s.store.Get(id)
	if !ok {
		return Permit{}, platform.NotFoundf("permit %q not found", id)
	}
	defer func() {
		// Audit is best-effort; never overwrite the returned business error.
		if s.audit != nil {
			_ = s.audit.Record(ctx, "operator", "complete_permit", "permit", id, "")
		}
	}()
	next, err := MustTransition(p.State, StateCompleted)
	if err != nil {
		return p, err
	}
	out, _ := s.store.Update(id, func(x *Permit) {
		x.State = next
		x.CompletedAt = s.clock.Now()
	})
	return out, nil
}

// Cancel moves a permit to cancelled with an optional reason.
func (s *Service) Cancel(ctx context.Context, id, reason string) (p Permit, err error) {
	p, ok := s.store.Get(id)
	if !ok {
		return Permit{}, platform.NotFoundf("permit %q not found", id)
	}
	defer func() {
		// Audit is best-effort; never overwrite the returned business error.
		if s.audit != nil {
			_ = s.audit.Record(ctx, "operator", "cancel_permit", "permit", id, reason)
		}
	}()
	next, err := MustTransition(p.State, StateCancelled)
	if err != nil {
		return p, err
	}
	out, _ := s.store.Update(id, func(x *Permit) {
		x.State = next
		x.CancelReason = reason
	})
	return out, nil
}

// ExpireScan marks approved permits whose window has passed without starting as
// expired. Returns the ids that were transitioned.
func (s *Service) ExpireScan(ctx context.Context) []string {
	now := s.clock.Now()
	var expired []string
	for _, p := range s.store.All() {
		if p.State != StateApproved {
			continue
		}
		if now.After(p.WindowEnd) {
			if _, err := s.expireOne(ctx, p.ID); err == nil {
				expired = append(expired, p.ID)
			}
		}
	}
	return expired
}

func (s *Service) expireOne(ctx context.Context, id string) (Permit, error) {
	p, ok := s.store.Get(id)
	if !ok {
		return Permit{}, platform.NotFoundf("permit %q not found", id)
	}
	next, err := MustTransition(p.State, StateExpired)
	if err != nil {
		return p, err
	}
	out, _ := s.store.Update(id, func(x *Permit) {
		x.State = next
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, "system", "expire_permit", "permit", id, "")
	}
	return out, nil
}

// Get returns a permit by id.
func (s *Service) Get(ctx context.Context, id string) (Permit, error) {
	p, ok := s.store.Get(id)
	if !ok {
		return Permit{}, platform.NotFoundf("permit %q not found", id)
	}
	return p, nil
}

// List returns all permits.
func (s *Service) List(ctx context.Context) []Permit {
	return s.store.All()
}
