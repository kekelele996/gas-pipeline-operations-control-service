package contract

// Contract service: create contracts, query remaining capacity, reserve and
// release capacity atomically. The nomination package calls Reserve/Release
// to implement the shipper nomination workflow.

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

// Service implements contract business logic.
type Service struct {
	store *Store
	clock platform.Clock
	audit AuditRecorder
}

// NewService builds a contract service.
func NewService(store *Store, clock platform.Clock, audit AuditRecorder) *Service {
	if clock == nil {
		clock = platform.SystemClock{}
	}
	return &Service{store: store, clock: clock, audit: audit}
}

// Store exposes the underlying store (read-only consumers).
func (s *Service) Store() *Store { return s.store }

// ContractInput is the payload for creating a contract.
type ContractInput struct {
	ID             string    `json:"id"`
	ShipperID      string    `json:"shipper_id"`
	ShipperName    string    `json:"shipper_name"`
	Code           string    `json:"code"`
	DailyVolume    float64   `json:"daily_volume"`
	ValidFrom      time.Time `json:"valid_from"`
	ValidTo        time.Time `json:"valid_to"`
	PathSegmentIDs []string  `json:"path_segment_ids"`
	Notes          string    `json:"notes"`
}

// Create validates and stores a new contract in the active state.
func (s *Service) Create(ctx context.Context, in ContractInput) (Contract, error) {
	v := platform.NewValidate()
	v.RequireNonEmpty("shipper_id", in.ShipperID)
	v.RequireNonEmpty("shipper_name", in.ShipperName)
	v.RequireNonEmpty("code", in.Code)
	v.RequirePositive("daily_volume", in.DailyVolume)
	if !in.ValidFrom.IsZero() && !in.ValidTo.IsZero() && !in.ValidTo.After(in.ValidFrom) {
		v.Require(false, "valid_to must be after valid_from")
	}
	if err := v.Error(); err != nil {
		return Contract{}, err
	}
	id := in.ID
	if id == "" {
		id = platform.NewContractID()
	}
	c := Contract{
		ID: id, ShipperID: in.ShipperID, ShipperName: in.ShipperName, Code: in.Code,
		DailyVolume: in.DailyVolume, ValidFrom: in.ValidFrom, ValidTo: in.ValidTo,
		PathSegmentIDs: in.PathSegmentIDs, Notes: in.Notes,
		State: StateActive,
	}
	s.store.Put(c)
	out, _ := s.store.Get(id)
	if s.audit != nil {
		_ = s.audit.Record(ctx, "system", "create_contract", "contract", id,
			fmt.Sprintf("%s daily=%.0f", in.ShipperName, in.DailyVolume))
	}
	return out, nil
}

// Get returns a contract by id.
func (s *Service) Get(ctx context.Context, id string) (Contract, error) {
	c, ok := s.store.Get(id)
	if !ok {
		return Contract{}, platform.NotFoundf("contract %q not found", id)
	}
	return c, nil
}

// List returns all contracts.
func (s *Service) List(ctx context.Context) []Contract {
	return s.store.All()
}

// ListByShipper returns a shipper's contracts.
func (s *Service) ListByShipper(ctx context.Context, shipperID string) []Contract {
	return s.store.ByShipper(shipperID)
}

// Remaining returns the remaining reservable capacity for a contract.
func (s *Service) Remaining(ctx context.Context, id string) (float64, error) {
	c, ok := s.store.Get(id)
	if !ok {
		return 0, platform.NotFoundf("contract %q not found", id)
	}
	return c.Remaining(), nil
}

// Reserve attempts to reserve amount m³ against a contract. It fails if the
// contract is not active, not in effect, or has insufficient remaining
// capacity. The reservation is atomic per contract.
func (s *Service) Reserve(ctx context.Context, id string, amount float64) (Contract, error) {
	if amount <= 0 {
		return Contract{}, platform.Invalidf("reserve amount must be positive")
	}
	cur, ok := s.store.Get(id)
	if !ok {
		return Contract{}, platform.NotFoundf("contract %q not found", id)
	}
	if !cur.IsActiveNow(s.clock.Now()) {
		return cur, platform.Statef("contract %q is not active now (state=%s)", id, cur.State)
	}
	if cur.Remaining() < amount {
		return cur, platform.Conflictf("contract %q has %.2f remaining, need %.2f",
			id, cur.Remaining(), amount)
	}
	out, _ := s.store.Update(id, func(x *Contract) {
		x.UsedVolume += amount
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, "nomination", "reserve_capacity", "contract", id,
			fmt.Sprintf("amount=%.2f remaining=%.2f", amount, out.Remaining()))
	}
	return out, nil
}

// Release returns reserved capacity back to a contract (e.g. on nomination
// cancellation). It will not drive used below zero.
func (s *Service) Release(ctx context.Context, id string, amount float64) (Contract, error) {
	if amount <= 0 {
		return Contract{}, platform.Invalidf("release amount must be positive")
	}
	if _, ok := s.store.Get(id); !ok {
		return Contract{}, platform.NotFoundf("contract %q not found", id)
	}
	out, _ := s.store.Update(id, func(x *Contract) {
		x.UsedVolume -= amount
		if x.UsedVolume < 0 {
			x.UsedVolume = 0
		}
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, "nomination", "release_capacity", "contract", id,
			fmt.Sprintf("amount=%.2f remaining=%.2f", amount, out.Remaining()))
	}
	return out, nil
}

// SetState moves a contract to a new state via the transition table.
func (s *Service) SetState(ctx context.Context, id string, target State) (Contract, error) {
	c, ok := s.store.Get(id)
	if !ok {
		return Contract{}, platform.NotFoundf("contract %q not found", id)
	}
	next, err := MustTransition(c.State, target)
	if err != nil {
		return c, err
	}
	out, _ := s.store.Update(id, func(x *Contract) {
		x.State = next
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, "operator", "contract_state", "contract", id, "-> "+target.String())
	}
	return out, nil
}

// ExpireDue scans contracts whose ValidTo has passed and marks them expired.
// Returns the ids that were transitioned.
func (s *Service) ExpireDue(ctx context.Context) []string {
	now := s.clock.Now()
	var expired []string
	for _, c := range s.store.All() {
		if c.State != StateActive {
			continue
		}
		if !c.ValidTo.IsZero() && now.After(c.ValidTo) {
			if _, err := s.SetState(ctx, c.ID, StateExpired); err == nil {
				expired = append(expired, c.ID)
			}
		}
	}
	return expired
}
