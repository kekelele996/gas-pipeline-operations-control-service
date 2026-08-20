package metering

import (
	"context"
	"errors"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/audit"
	"gas-pipeline-operations-control-service/internal/platform"
)

func newSvcR003() *Service {
	c := platform.SystemClock{}
	auditSvc := audit.NewService(audit.NewStore(100), c)
	return NewService(NewStore(5000), c, auditSvc)
}

// TestMeteringGetSettlementMissingR003A: missing settlements must stay
// not-found so callers can classify them with errors.Is.
func TestMeteringGetSettlementMissingR003A(t *testing.T) {
	svc := newSvcR003()
	_, err := svc.GetSettlement(context.Background(), "2030-01-01")
	if err == nil {
		t.Fatal("expected error for missing settlement")
	}
	if !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("missing settlement not classified as not-found: %v", err)
	}
}

// TestMeteringConfirmMissingR003B: confirming a missing settlement must stay
// not-found.
func TestMeteringConfirmMissingR003B(t *testing.T) {
	svc := newSvcR003()
	_, err := svc.Confirm(context.Background(), "2030-01-01", "ops")
	if err == nil {
		t.Fatal("expected error for missing settlement")
	}
	if !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("confirm missing settlement not classified as not-found: %v", err)
	}
}

// TestMeteringReconcileMissingR003C: reconciling a missing settlement must
// stay not-found.
func TestMeteringReconcileMissingR003C(t *testing.T) {
	svc := newSvcR003()
	_, err := svc.Reconcile(context.Background(), "2030-01-01", "ops")
	if err == nil {
		t.Fatal("expected error for missing settlement")
	}
	if !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("reconcile missing settlement not classified as not-found: %v", err)
	}
}

// TestMeteringReadingMissingMeterR003F: recording a reading for an unknown
// meter must stay not-found.
func TestMeteringReadingMissingMeterR003F(t *testing.T) {
	svc := newSvcR003()
	_, err := svc.RecordReading(context.Background(), RawReading{MeterID: "NOPE", Value: 100, Ts: time.Now()})
	if err == nil {
		t.Fatal("expected error for unknown meter")
	}
	if !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("unknown meter not classified as not-found: %v", err)
	}
}

// TestMeteringSettleEmptyDateR003G: an empty settle date must be invalid
// input, not a generic system failure.
func TestMeteringSettleEmptyDateR003G(t *testing.T) {
	svc := newSvcR003()
	_, err := svc.SettleDaily(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty date")
	}
	if !errors.Is(err, platform.ErrInvalid) {
		t.Fatalf("empty date not classified as invalid: %v", err)
	}
}

// TestMeteringGetMeterMissingR003J: fetching an unknown meter must stay
// not-found.
func TestMeteringGetMeterMissingR003J(t *testing.T) {
	svc := newSvcR003()
	_, err := svc.GetMeter(context.Background(), "NOPE")
	if err == nil {
		t.Fatal("expected error for unknown meter")
	}
	if !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("unknown meter not classified as not-found: %v", err)
	}
}
