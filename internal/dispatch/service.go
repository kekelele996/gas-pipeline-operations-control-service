package dispatch

// Dispatch service: create orders (validating the target device exists),
// issue them (validating the device is in a state that allows the action),
// execute them (mutating the device via the network service), and revoke.

import (
	"context"
	"fmt"

	"gas-pipeline-operations-control-service/internal/network"
	"gas-pipeline-operations-control-service/internal/platform"
)

// AuditRecorder mirrors the network package's recorder interface.
type AuditRecorder interface {
	Record(ctx context.Context, actor, action, targetType, targetID, detail string) error
}

// NetworkService is the device surface the dispatch service drives. The
// network service implements it.
type NetworkService interface {
	GetCompressor(ctx context.Context, id string) (network.Compressor, error)
	ChangeCompressorState(ctx context.Context, id string, target network.CompressorState, reason string) (network.Compressor, error)
	GetValve(ctx context.Context, id string) (network.Valve, error)
	ChangeValveState(ctx context.Context, id string, target network.ValveState) (network.Valve, error)
}

// Service implements dispatch business logic.
type Service struct {
	store   *Store
	clock   platform.Clock
	network NetworkService
	audit   AuditRecorder
}

// NewService builds a dispatch service.
func NewService(store *Store, ns NetworkService, clock platform.Clock, audit AuditRecorder) *Service {
	if clock == nil {
		clock = platform.SystemClock{}
	}
	return &Service{store: store, clock: clock, network: ns, audit: audit}
}

// Store exposes the underlying store (read-only consumers; permit service
// uses PendingOrdersForSegment via Store directly).
func (s *Service) Store() *Store { return s.store }

// Input is the payload for creating an order.
type Input struct {
	Type     Type    `json:"type"`
	TargetID string  `json:"target_id"`
	Value    float64 `json:"value"`
	Priority int     `json:"priority"`
	Reason   string  `json:"reason"`
}

// Create validates and stores a new order in the pending state. The target
// device must exist; its segment is recorded for conflict checks.
func (s *Service) Create(ctx context.Context, in Input) (Order, error) {
	v := platform.NewValidate()
	v.RequireNonEmpty("target_id", in.TargetID)
	v.RequireNonEmpty("reason", in.Reason)
	validType := false
	for _, t := range AllTypes() {
		if t == in.Type {
			validType = true
			break
		}
	}
	v.Require(validType, "type is invalid")
	if err := v.Error(); err != nil {
		return Order{}, err
	}
	targetType := TargetTypeFor(in.Type)
	segmentID, err := s.resolveSegment(ctx, targetType, in.TargetID)
	if err != nil {
		return Order{}, err
	}
	o := Order{
		ID: platform.NewOrderID(), Type: in.Type, TargetType: targetType,
		TargetID: in.TargetID, SegmentID: segmentID, Value: in.Value,
		Priority: in.Priority, Reason: in.Reason, State: StatePending,
	}
	s.store.Put(o)
	out, _ := s.store.Get(o.ID)
	if s.audit != nil {
		_ = s.audit.Record(ctx, "dispatcher", "create_order", "order", o.ID,
			fmt.Sprintf("%s on %s", in.Type, in.TargetID))
	}
	return out, nil
}

// resolveSegment looks up the target device to validate existence and find its
// segment.
func (s *Service) resolveSegment(ctx context.Context, targetType, id string) (string, error) {
	switch targetType {
	case "compressor":
		c, err := s.network.GetCompressor(ctx, id)
		if err != nil {
			return "", err
		}
		return c.SegmentID, nil
	case "valve":
		v, err := s.network.GetValve(ctx, id)
		if err != nil {
			return "", err
		}
		return v.SegmentID, nil
	default:
		// flow orders target a meter/segment directly; treat the id as the segment
		return id, nil
	}
}

// Issue moves a pending order to issued, after validating the device is in a
// state that permits the action (e.g. a close_valve order requires the valve
// to not already be closed).
func (s *Service) Issue(ctx context.Context, id, by string) (Order, error) {
	o, ok := s.store.Get(id)
	if !ok {
		return Order{}, platform.NotFoundf("order %q not found", id)
	}
	if _, err := MustTransition(o.State, StateIssued); err != nil {
		return o, err
	}
	if err := s.checkActionAllowed(ctx, o); err != nil {
		return o, err
	}
	out, _ := s.store.Update(id, func(x *Order) {
		x.State = StateIssued
		x.IssuedBy = by
		x.IssuedAt = s.clock.Now()
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, by, "issue_order", "order", id, "")
	}
	return out, nil
}

// checkActionAllowed verifies the device is in a state that permits the order's
// action, returning an ErrState if not.
func (s *Service) checkActionAllowed(ctx context.Context, o Order) error {
	switch o.Type {
	case TypeOpenValve:
		v, err := s.network.GetValve(ctx, o.TargetID)
		if err != nil {
			return err
		}
		if v.State == network.ValveOpen {
			return platform.Statef("valve %s is already open", o.TargetID)
		}
	case TypeCloseValve:
		v, err := s.network.GetValve(ctx, o.TargetID)
		if err != nil {
			return err
		}
		if v.State == network.ValveClosed {
			return platform.Statef("valve %s is already closed", o.TargetID)
		}
	case TypeStartCompressor:
		c, err := s.network.GetCompressor(ctx, o.TargetID)
		if err != nil {
			return err
		}
		if c.State == network.CompressorRunning {
			return platform.Statef("compressor %s is already running", o.TargetID)
		}
		if c.State == network.CompressorMaintenance {
			return platform.Statef("compressor %s is under maintenance", o.TargetID)
		}
	case TypeStopCompressor:
		c, err := s.network.GetCompressor(ctx, o.TargetID)
		if err != nil {
			return err
		}
		if c.State == network.CompressorStopped {
			return platform.Statef("compressor %s is already stopped", o.TargetID)
		}
	}
	return nil
}

// Execute moves an issued order to executed and drives the device state change
// through the network service.
func (s *Service) Execute(ctx context.Context, id string) (Order, error) {
	o, ok := s.store.Get(id)
	if !ok {
		return Order{}, platform.NotFoundf("order %q not found", id)
	}
	next, err := MustTransition(o.State, StateExecuted)
	if err != nil {
		return o, err
	}
	outcome, err := s.apply(ctx, o)
	if err != nil {
		return o, err
	}
	out, _ := s.store.Update(id, func(x *Order) {
		x.State = next
		x.ExecutedAt = s.clock.Now()
		x.Outcome = outcome
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, "system", "execute_order", "order", id, outcome)
	}
	return out, nil
}

// apply performs the device state change the order commands.
func (s *Service) apply(ctx context.Context, o Order) (string, error) {
	switch o.Type {
	case TypeOpenValve:
		if _, err := s.network.ChangeValveState(ctx, o.TargetID, network.ValveOpen); err != nil {
			return "", err
		}
		return "valve opened", nil
	case TypeCloseValve:
		if _, err := s.network.ChangeValveState(ctx, o.TargetID, network.ValveClosed); err != nil {
			return "", err
		}
		return "valve closed", nil
	case TypeStartCompressor:
		if _, err := s.network.ChangeCompressorState(ctx, o.TargetID, network.CompressorRunning, o.Reason); err != nil {
			return "", err
		}
		return "compressor started", nil
	case TypeStopCompressor:
		if _, err := s.network.ChangeCompressorState(ctx, o.TargetID, network.CompressorStopped, o.Reason); err != nil {
			return "", err
		}
		return "compressor stopped", nil
	case TypeRaiseFlow, TypeLowerFlow:
		return fmt.Sprintf("flow set to %.2f", o.Value), nil
	}
	return "", platform.Invalidf("unknown order type %q", o.Type)
}

// Revoke moves a pending or issued order to revoked.
func (s *Service) Revoke(ctx context.Context, id string) (Order, error) {
	o, ok := s.store.Get(id)
	if !ok {
		return Order{}, platform.NotFoundf("order %q not found", id)
	}
	next, err := MustTransition(o.State, StateRevoked)
	if err != nil {
		return o, err
	}
	out, _ := s.store.Update(id, func(x *Order) {
		x.State = next
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, "dispatcher", "revoke_order", "order", id, "")
	}
	return out, nil
}

// Get returns an order by id.
func (s *Service) Get(ctx context.Context, id string) (Order, error) {
	o, ok := s.store.Get(id)
	if !ok {
		return Order{}, platform.NotFoundf("order %q not found", id)
	}
	return o, nil
}

// List returns all orders.
func (s *Service) List(ctx context.Context) []Order {
	return s.store.All()
}

// PendingOrdersForSegment implements permit.DispatchConflictChecker.
func (s *Service) PendingOrdersForSegment(segmentID string) []string {
	var ids []string
	for _, o := range s.store.PendingForSegment(segmentID) {
		ids = append(ids, o.ID)
	}
	return ids
}
