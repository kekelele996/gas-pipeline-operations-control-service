package network

import (
	"context"
	"testing"

	"gas-pipeline-operations-control-service/internal/audit"
	"gas-pipeline-operations-control-service/internal/config"
	"gas-pipeline-operations-control-service/internal/platform"
)

func newSvcR004(t *testing.T) *Service {
	t.Helper()
	auditSvc := audit.NewService(audit.NewStore(100), platform.SystemClock{})
	svc := NewService(NewStore(), platform.SystemClock{}, auditSvc)
	// default (zero-value) config holds a typed-nil limit provider
	svc.SetLimitProvider(config.Default().LimitProvider)
	if _, err := svc.UpsertSegment(context.Background(), SegmentInput{
		ID: "SEG-1", Name: "trunk", From: "A", To: "B",
		LengthKm: 100, DiameterMm: 800, MAOPMPa: 6.3, Tier: "main",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpsertStation(context.Background(), StationInput{
		ID: "ST-1", Name: "North", Code: "N", Type: StationSource,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpsertValve(context.Background(), ValveInput{
		ID: "V-1", Name: "block", SegmentID: "SEG-1", StationID: "ST-1",
		Type: "block", Normal: "open",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpsertCompressor(context.Background(), CompressorInput{
		ID: "C-1", Name: "comp", StationID: "ST-1", SegmentID: "SEG-1",
		PowerMW: 5.0, State: CompressorStopped,
	}); err != nil {
		t.Fatal(err)
	}
	return svc
}

// TestNetworkRecordLimitNoPanicR004B: recording an operating limit under the
// default config must not panic on a nil provider map.
func TestNetworkRecordLimitNoPanicR004B(t *testing.T) {
	svc := newSvcR004(t)
	if err := svc.RecordOperatingLimit(context.Background(), "SEG-1", 6.0); err != nil {
		t.Fatalf("record limit: %v", err)
	}
	v, ok := svc.Store().SegmentLimit("SEG-1")
	if !ok || v != 6.0 {
		t.Fatalf("limit not stored: ok=%v v=%g", ok, v)
	}
}

// TestNetworkRecordStationLimitNoPanicR004C: recording a station limit under
// the default config must not panic either.
func TestNetworkRecordStationLimitNoPanicR004C(t *testing.T) {
	svc := newSvcR004(t)
	if err := svc.RecordStationLimit(context.Background(), "ST-1", 6.2); err != nil {
		t.Fatalf("record station limit: %v", err)
	}
}

// TestNetworkStoreLimitInitR004D: a freshly built store must accept limit
// writes without panicking.
func TestNetworkStoreLimitInitR004D(t *testing.T) {
	st := NewStore()
	st.PutSegmentLimit("SEG-9", 5.5)
	v, ok := st.SegmentLimit("SEG-9")
	if !ok || v != 5.5 {
		t.Fatalf("limit not stored: ok=%v v=%g", ok, v)
	}
}

// TestNetworkApplySegmentLimitsR004E: batch-applying segment limits must not
// panic and must record the limits.
func TestNetworkApplySegmentLimitsR004E(t *testing.T) {
	svc := newSvcR004(t)
	n, err := svc.ApplySegmentLimits(context.Background(), map[string]float64{"SEG-1": 6.1})
	if err != nil {
		t.Fatalf("apply limits: %v", err)
	}
	if n != 1 {
		t.Fatalf("applied %d limits, want 1", n)
	}
}

// TestNetworkRecordValveLimitNoPanicR004I: recording a valve limit under the
// default config must not panic.
func TestNetworkRecordValveLimitNoPanicR004I(t *testing.T) {
	svc := newSvcR004(t)
	if err := svc.RecordValveLimit(context.Background(), "V-1", 5.0); err != nil {
		t.Fatalf("record valve limit: %v", err)
	}
	v, ok := svc.Store().ValveLimit("V-1")
	if !ok || v != 5.0 {
		t.Fatalf("valve limit not stored: ok=%v v=%g", ok, v)
	}
}

// TestNetworkRecordCompressorLimitNoPanicR004J: recording a compressor limit
// under the default config must not panic.
func TestNetworkRecordCompressorLimitNoPanicR004J(t *testing.T) {
	svc := newSvcR004(t)
	if err := svc.RecordCompressorLimit(context.Background(), "C-1", 7.5); err != nil {
		t.Fatalf("record compressor limit: %v", err)
	}
	v, ok := svc.Store().CompressorLimit("C-1")
	if !ok || v != 7.5 {
		t.Fatalf("compressor limit not stored: ok=%v v=%g", ok, v)
	}
}

// TestNetworkOperatingLimitsSafeR004K: the provider limit map exposed to
// callers must be writable (non-nil) under the default config.
func TestNetworkOperatingLimitsSafeR004K(t *testing.T) {
	svc := newSvcR004(t)
	m := svc.OperatingLimits()
	if m == nil {
		t.Fatal("OperatingLimits returned nil map under default config")
	}
	m["SEG-1"] = 6.5
}
