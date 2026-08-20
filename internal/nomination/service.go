package nomination

// Nomination service: submit nominations against contracts (reserving
// capacity), confirm, execute (debit contract used), and cancel (release
// capacity). All capacity operations go through the contract service so they
// are atomic per contract.

import (
	"context"
	"fmt"
	"time"

	"gas-pipeline-operations-control-service/internal/contract"
	"gas-pipeline-operations-control-service/internal/platform"
)

// AuditRecorder mirrors the network package's recorder interface.
type AuditRecorder interface {
	Record(ctx context.Context, actor, action, targetType, targetID, detail string) error
}

// ContractService is the capacity surface the nomination service needs.
type ContractService interface {
	Remaining(ctx context.Context, id string) (float64, error)
	Reserve(ctx context.Context, id string, amount float64) (contract.Contract, error)
	Release(ctx context.Context, id string, amount float64) (contract.Contract, error)
	Get(ctx context.Context, id string) (contract.Contract, error)
}

// Service implements nomination business logic.
type Service struct {
	store     *Store
	clock     platform.Clock
	contracts ContractService
	audit     AuditRecorder
}

// NewService builds a nomination service.
func NewService(store *Store, contracts ContractService, clock platform.Clock, audit AuditRecorder) *Service {
	if clock == nil {
		clock = platform.SystemClock{}
	}
	return &Service{store: store, contracts: contracts, clock: clock, audit: audit}
}

// Store exposes the underlying store (read-only consumers).
func (s *Service) Store() *Store { return s.store }

// Input is the payload for creating a nomination.
type Input struct {
	ContractID  string   `json:"contract_id"`
	Date        string   `json:"date"`
	Volume      float64  `json:"volume"`
	Path        []string `json:"path"`
	SubmittedBy string   `json:"submitted_by"`
	Note        string   `json:"note"`
}

// Create drafts a new nomination in the draft state without reserving
// capacity. Capacity is reserved only on Submit.
func (s *Service) Create(ctx context.Context, in Input) (Nomination, error) {
	v := platform.NewValidate()
	v.RequireNonEmpty("contract_id", in.ContractID)
	v.RequireNonEmpty("date", in.Date)
	v.RequirePositive("volume", in.Volume)
	if _, err := time.Parse("2006-01-02", in.Date); err != nil {
		v.Require(false, "date must be YYYY-MM-DD")
	}
	if err := v.Error(); err != nil {
		return Nomination{}, err
	}
	c, err := s.contracts.Get(ctx, in.ContractID)
	if err != nil {
		return Nomination{}, err
	}
	n := Nomination{
		ID: platform.NewNominationID(), ContractID: in.ContractID,
		ShipperID: c.ShipperID, Date: in.Date, Volume: in.Volume,
		Path: in.Path, State: StateDraft, SubmittedBy: in.SubmittedBy,
		Note: in.Note,
	}
	s.store.Put(n)
	out, _ := s.store.Get(n.ID)
	if s.audit != nil {
		_ = s.audit.Record(ctx, in.SubmittedBy, "create_nomination", "nomination", n.ID,
			fmt.Sprintf("contract=%s date=%s vol=%.0f", in.ContractID, in.Date, in.Volume))
	}
	return out, nil
}

// Submit moves a draft nomination to submitted and reserves contract capacity.
// It fails atomically if the contract has insufficient remaining capacity.
func (s *Service) Submit(ctx context.Context, id, by string) (Nomination, error) {
	n, ok := s.store.Get(id)
	if !ok {
		return Nomination{}, platform.NotFoundf("nomination %q not found", id)
	}
	if _, err := MustTransition(n.State, StateSubmitted); err != nil {
		return n, err
	}
	// reserve capacity — atomic per contract via contract service
	if _, err := s.contracts.Reserve(ctx, n.ContractID, n.Volume); err != nil {
		return n, err
	}
	out, _ := s.store.Update(id, func(x *Nomination) {
		x.State = StateSubmitted
		x.Reserved = true
		x.SubmittedBy = by
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, by, "submit_nomination", "nomination", id,
			fmt.Sprintf("reserved %.0f against %s", n.Volume, n.ContractID))
	}
	return out, nil
}

// Confirm moves a submitted nomination to confirmed.
func (s *Service) Confirm(ctx context.Context, id, by string) (Nomination, error) {
	n, ok := s.store.Get(id)
	if !ok {
		return Nomination{}, platform.NotFoundf("nomination %q not found", id)
	}
	next, err := MustTransition(n.State, StateConfirmed)
	if err != nil {
		return n, err
	}
	out, _ := s.store.Update(id, func(x *Nomination) {
		x.State = next
		x.ConfirmedBy = by
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, by, "confirm_nomination", "nomination", id, "")
	}
	return out, nil
}

// Execute moves a confirmed nomination to executed. Capacity remains reserved
// (already debited on submit) — execution marks the nomination as fulfilled
// for the gas day.
func (s *Service) Execute(ctx context.Context, id string) (Nomination, error) {
	n, ok := s.store.Get(id)
	if !ok {
		return Nomination{}, platform.NotFoundf("nomination %q not found", id)
	}
	next, err := MustTransition(n.State, StateExecuted)
	if err != nil {
		return n, err
	}
	out, _ := s.store.Update(id, func(x *Nomination) {
		x.State = next
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, "system", "execute_nomination", "nomination", id, "")
	}
	return out, nil
}

// Cancel moves a nomination to cancelled and releases any held capacity.
func (s *Service) Cancel(ctx context.Context, id string) (Nomination, error) {
	n, ok := s.store.Get(id)
	if !ok {
		return Nomination{}, platform.NotFoundf("nomination %q not found", id)
	}
	if _, err := MustTransition(n.State, StateCancelled); err != nil {
		return n, err
	}
	if n.Reserved {
		if _, err := s.contracts.Release(ctx, n.ContractID, n.Volume); err != nil {
			// capacity release failed — record but still cancel the nomination;
			// the contract used-volume may be slightly overstated, which is
			// safer than leaking the nomination.
			if s.audit != nil {
				_ = s.audit.Record(ctx, "system", "cancel_release_failed", "nomination", id, err.Error())
			}
		}
	}
	out, _ := s.store.Update(id, func(x *Nomination) {
		x.State = StateCancelled
		x.Reserved = false
	})
	if s.audit != nil {
		_ = s.audit.Record(ctx, "operator", "cancel_nomination", "nomination", id, "")
	}
	return out, nil
}

// Get returns a nomination by id.
func (s *Service) Get(ctx context.Context, id string) (Nomination, error) {
	n, ok := s.store.Get(id)
	if !ok {
		return Nomination{}, platform.NotFoundf("nomination %q not found", id)
	}
	return n, nil
}

// List returns all nominations.
func (s *Service) List(ctx context.Context) []Nomination {
	return s.store.All()
}

// ListForContractDate returns nominations for a contract on a date.
func (s *Service) ListForContractDate(ctx context.Context, contractID, date string) []Nomination {
	return s.store.ForContractDate(contractID, date)
}
