package incident

// Incident service: report (optionally from an alarm), confirm, add/complete
// action items, and close. Closing requires all action items complete; the
// service can auto-resolve the originating alarm when an incident closes.

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

// AlarmResolver is implemented by the SCADA service so an incident closing can
// resolve its originating alarm.
type AlarmResolver interface {
	ResolveAlarm(ctx context.Context, id string) error
}

// Service implements incident business logic.
type Service struct {
	store  *Store
	clock  platform.Clock
	audit  AuditRecorder
	alarms AlarmResolver
}

// NewService builds an incident service. alarms may be nil.
func NewService(store *Store, clock platform.Clock, audit AuditRecorder, alarms AlarmResolver) *Service {
	if clock == nil {
		clock = platform.SystemClock{}
	}
	return &Service{store: store, clock: clock, audit: audit, alarms: alarms}
}

// Store exposes the underlying store (read-only consumers; permit service
// uses OpenIncidentsForSegment).
func (s *Service) Store() *Store { return s.store }

// Input is the payload for reporting an incident.
type Input struct {
	SegmentID   string   `json:"segment_id"`
	DeviceID    string   `json:"device_id"`
	StationID   string   `json:"station_id"`
	Severity    Severity `json:"severity"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	AlarmIDs    []string `json:"alarm_ids"`
	Reporter    string   `json:"reporter"`
}

// Report creates a new incident in the pending state. When alarm ids are
// provided the originating alarms are associated for later resolution.
func (s *Service) Report(ctx context.Context, in Input) (Incident, error) {
	v := platform.NewValidate()
	v.RequireNonEmpty("title", in.Title)
	v.RequireNonEmpty("reporter", in.Reporter)
	validSev := false
	for _, sv := range AllSeverities() {
		if sv == in.Severity {
			validSev = true
			break
		}
	}
	v.Require(validSev, "severity is invalid")
	if in.SegmentID == "" && in.DeviceID == "" && in.StationID == "" {
		v.Require(false, "at least one of segment_id/device_id/station_id is required")
	}
	if err := v.Error(); err != nil {
		return Incident{}, err
	}
	i := Incident{
		ID: platform.NewIncidentID(), SegmentID: in.SegmentID, DeviceID: in.DeviceID,
		StationID: in.StationID, Severity: in.Severity, Title: in.Title,
		Description: in.Description, AlarmIDs: in.AlarmIDs, Reporter: in.Reporter,
		State: StatePending,
	}
	s.store.Put(i)
	out, _ := s.store.Get(i.ID)
	if s.audit != nil {
		_ = s.audit.Record(ctx, in.Reporter, "report_incident", "incident", i.ID,
			fmt.Sprintf("%s %s on %s", in.Severity, in.Title, in.SegmentID))
	}
	return out, nil
}

// Confirm moves a pending incident to handling and assigns an owner.
func (s *Service) Confirm(ctx context.Context, id, assignee string) (Incident, error) {
	i, ok := s.store.Get(id)
	if !ok {
		return Incident{}, platform.NotFoundf("incident %q not found", id)
	}
	next, err := MustTransition(i.State, StateHandling)
	if err != nil {
		return i, err
	}
	out, _ := s.store.Update(id, func(x *Incident) {
		x.State = next
		x.Assignee = assignee
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, assignee, "confirm_incident", "incident", id, "")
	}
	return out, nil
}

// AddAction appends a remediation action item to an incident.
func (s *Service) AddAction(ctx context.Context, id, description, owner string) (Incident, error) {
	i, ok := s.store.Get(id)
	if !ok {
		return Incident{}, platform.NotFoundf("incident %q not found", id)
	}
	if i.State != StateHandling {
		return i, platform.Statef("incident must be in handling state to add actions")
	}
	item := ActionItem{
		ID: platform.NewPrefixedID("ACT"), Description: description, Owner: owner,
	}
	out, _ := s.store.Update(id, func(x *Incident) {
		x.Actions = append(x.Actions, item)
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, owner, "add_action", "incident", id, description)
	}
	return out, nil
}

// CompleteAction marks an action item done.
func (s *Service) CompleteAction(ctx context.Context, id, actionID string) (Incident, error) {
	i, ok := s.store.Get(id)
	if !ok {
		return Incident{}, platform.NotFoundf("incident %q not found", id)
	}
	found := false
	out, _ := s.store.Update(id, func(x *Incident) {
		for idx := range x.Actions {
			if x.Actions[idx].ID == actionID {
				x.Actions[idx].Done = true
				x.Actions[idx].CompletedAt = s.clock.Now()
				found = true
				break
			}
		}
	})
	if !found {
		return i, platform.NotFoundf("action %q not found on incident %q", actionID, id)
	}
	if s.audit != nil {
		_ = s.audit.Record(ctx, "operator", "complete_action", "incident", id, actionID)
	}
	return out, nil
}

// Close moves a handling incident to closed, requiring all action items be
// complete. On close the originating alarms are resolved.
func (s *Service) Close(ctx context.Context, id string) (Incident, error) {
	i, ok := s.store.Get(id)
	if !ok {
		return Incident{}, platform.NotFoundf("incident %q not found", id)
	}
	next, err := MustTransition(i.State, StateClosed)
	if err != nil {
		return i, err
	}
	if !i.AllActionsComplete() {
		return i, platform.Conflictf("cannot close incident %q: not all actions complete", id)
	}
	out, _ := s.store.Update(id, func(x *Incident) {
		x.State = next
		x.ClosedAt = s.clock.Now()
	})
	// resolve originating alarms
	if s.alarms != nil {
		for _, aid := range i.AlarmIDs {
			_ = s.alarms.ResolveAlarm(ctx, aid)
		}
	}
	if s.audit != nil {
		_ = s.audit.Record(ctx, "operator", "close_incident", "incident", id, "")
	}
	return out, nil
}

// Get returns an incident by id.
func (s *Service) Get(ctx context.Context, id string) (Incident, error) {
	i, ok := s.store.Get(id)
	if !ok {
		return Incident{}, platform.NotFoundf("incident %q not found", id)
	}
	return i, nil
}

// List returns all incidents.
func (s *Service) List(ctx context.Context) []Incident {
	return s.store.All()
}

// OpenIncidentsForSegment implements permit.IncidentConflictChecker.
func (s *Service) OpenIncidentsForSegment(segmentID string) []string {
	var ids []string
	for _, i := range s.store.OpenForSegment(segmentID) {
		ids = append(ids, i.ID)
	}
	return ids
}

// OpenCount returns the number of open incidents.
func (s *Service) OpenCount(ctx context.Context) int {
	return s.store.OpenCount()
}

// now helper to keep time import
var _ = time.Now
