package contract

import (
	"context"
	"errors"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/audit"
	"gas-pipeline-operations-control-service/internal/platform"
)

func newSvcR009(t *testing.T) *Service {
	t.Helper()
	auditSvc := audit.NewService(audit.NewStore(100), platform.SystemClock{})
	return NewService(NewStore(), platform.SystemClock{}, auditSvc)
}

// TestContractReserveExhaustedChainR009A: reserving beyond remaining capacity
// must stay classified as exhausted.
func TestContractReserveExhaustedChainR009A(t *testing.T) {
	svc := newSvcR009(t)
	now := time.Now()
	c, err := svc.Create(context.Background(), ContractInput{
		ShipperID: "SH", ShipperName: "s", Code: "C1", DailyVolume: 100,
		ValidFrom: now.Add(-time.Hour), ValidTo: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Reserve(context.Background(), c.ID, 150)
	if err == nil {
		t.Fatal("expected reserve error")
	}
	if !errors.Is(err, platform.ErrExhausted) {
		t.Fatalf("over-capacity error = %v, want exhausted classification", err)
	}
}

// TestContractReserveNotActiveR009B: reserving on an inactive contract must
// stay classified as a state error.
func TestContractReserveNotActiveR009B(t *testing.T) {
	svc := newSvcR009(t)
	now := time.Now()
	c, err := svc.Create(context.Background(), ContractInput{
		ShipperID: "SH", ShipperName: "s", Code: "C2", DailyVolume: 100,
		ValidFrom: now.Add(-time.Hour), ValidTo: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetState(context.Background(), c.ID, StateSuspended); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Reserve(context.Background(), c.ID, 10)
	if err == nil {
		t.Fatal("expected reserve error")
	}
	if !errors.Is(err, platform.ErrState) {
		t.Fatalf("inactive reserve error = %v, want state classification", err)
	}
}

// TestContractReserveMissingR009E: reserving on an unknown contract must stay
// not-found.
func TestContractReserveMissingR009E(t *testing.T) {
	svc := newSvcR009(t)
	_, err := svc.Reserve(context.Background(), "NOPE", 10)
	if err == nil {
		t.Fatal("expected reserve error")
	}
	if !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("missing contract error = %v, want not-found", err)
	}
}

// TestContractRemainingMissingR009L: querying remaining capacity of an
// unknown contract must stay classified as not-found.
func TestContractRemainingMissingR009L(t *testing.T) {
	svc := newSvcR009(t)
	_, err := svc.Remaining(context.Background(), "NOPE")
	if err == nil {
		t.Fatal("expected remaining error")
	}
	if !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("missing remaining error = %v, want not-found", err)
	}
}

// TestContractReleaseMissingR009M: releasing capacity on an unknown contract
// must stay classified as not-found.
func TestContractReleaseMissingR009M(t *testing.T) {
	svc := newSvcR009(t)
	_, err := svc.Release(context.Background(), "NOPE", 10)
	if err == nil {
		t.Fatal("expected release error")
	}
	if !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("missing release error = %v, want not-found", err)
	}
}
