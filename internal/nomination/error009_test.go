package nomination

import (
	"context"
	"errors"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/audit"
	"gas-pipeline-operations-control-service/internal/contract"
	"gas-pipeline-operations-control-service/internal/platform"
)

func newSvcR009(t *testing.T) (*Service, *contract.Service, contract.Contract) {
	t.Helper()
	auditSvc := audit.NewService(audit.NewStore(100), platform.SystemClock{})
	ctr := contract.NewService(contract.NewStore(), platform.SystemClock{}, auditSvc)
	now := time.Now()
	c, err := ctr.Create(context.Background(), contract.ContractInput{
		ShipperID: "SH", ShipperName: "s", Code: "C1", DailyVolume: 100,
		ValidFrom: now.Add(-time.Hour), ValidTo: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(NewStore(), ctr, platform.SystemClock{}, auditSvc)
	return svc, ctr, c
}

// TestNominationSubmitOverCapacityR009D: submitting a nomination beyond the
// contract's remaining capacity must stay classified as exhausted.
func TestNominationSubmitOverCapacityR009D(t *testing.T) {
	svc, _, c := newSvcR009(t)
	n, err := svc.Create(context.Background(), Input{
		ContractID: c.ID, Date: "2026-09-01", Volume: 150,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Submit(context.Background(), n.ID, "shipper")
	if err == nil {
		t.Fatal("expected submit error")
	}
	if !errors.Is(err, platform.ErrExhausted) {
		t.Fatalf("over-capacity submit error = %v, want exhausted classification", err)
	}
}

// TestNominationSubmitMissingR009F: submitting an unknown nomination must
// stay not-found.
func TestNominationSubmitMissingR009F(t *testing.T) {
	svc, _, _ := newSvcR009(t)
	_, err := svc.Submit(context.Background(), "NOPE", "shipper")
	if err == nil {
		t.Fatal("expected submit error")
	}
	if !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("missing nomination error = %v, want not-found", err)
	}
}

// TestNominationSubmitIllegalStateR009H: submitting an already-submitted
// nomination must stay classified as a state error.
func TestNominationSubmitIllegalStateR009H(t *testing.T) {
	svc, _, c := newSvcR009(t)
	n, err := svc.Create(context.Background(), Input{ContractID: c.ID, Date: "2026-09-03", Volume: 10})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Submit(context.Background(), n.ID, "shipper"); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Submit(context.Background(), n.ID, "shipper")
	if err == nil {
		t.Fatal("expected second submit error")
	}
	if !errors.Is(err, platform.ErrState) {
		t.Fatalf("illegal submit error = %v, want state classification", err)
	}
}

// TestNominationConfirmIllegalStateR009I: confirming a non-submitted
// nomination must stay classified as a state error.
func TestNominationConfirmIllegalStateR009I(t *testing.T) {
	svc, _, c := newSvcR009(t)
	n, err := svc.Create(context.Background(), Input{ContractID: c.ID, Date: "2026-09-04", Volume: 10})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Confirm(context.Background(), n.ID, "ops")
	if err == nil {
		t.Fatal("expected confirm error")
	}
	if !errors.Is(err, platform.ErrState) {
		t.Fatalf("illegal confirm error = %v, want state classification", err)
	}
}

// TestNominationExecuteIllegalStateR009J: executing a non-confirmed
// nomination must stay classified as a state error.
func TestNominationExecuteIllegalStateR009J(t *testing.T) {
	svc, _, c := newSvcR009(t)
	n, err := svc.Create(context.Background(), Input{ContractID: c.ID, Date: "2026-09-05", Volume: 10})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Execute(context.Background(), n.ID)
	if err == nil {
		t.Fatal("expected execute error")
	}
	if !errors.Is(err, platform.ErrState) {
		t.Fatalf("illegal execute error = %v, want state classification", err)
	}
}

// TestNominationCreateMissingContractR009K: creating a nomination against an
// unknown contract must stay classified as not-found.
func TestNominationCreateMissingContractR009K(t *testing.T) {
	svc, _, _ := newSvcR009(t)
	_, err := svc.Create(context.Background(), Input{ContractID: "NOPE", Date: "2026-09-06", Volume: 10})
	if err == nil {
		t.Fatal("expected create error")
	}
	if !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("missing contract create error = %v, want not-found", err)
	}
}
