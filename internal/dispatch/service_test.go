package dispatch

import (
	"context"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/network"
	"gas-pipeline-operations-control-service/internal/platform"
)

type env struct {
	dispatch *Service
	network  *network.Service
	clock    *platform.FakeClock
}

func newEnv(t *testing.T) *env {
	t.Helper()
	c := platform.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	nstore := network.NewStore()
	ns := network.NewService(nstore, c, nil)
	ds := NewService(NewStore(), ns, c, nil)
	// seed a segment, station, compressor, valve
	seg, _ := ns.UpsertSegment(context.Background(), network.SegmentInput{
		Name: "Trunk", From: "S1", To: "S2", LengthKm: 10, DiameterMm: 500, MAOPMPa: 9, Tier: "main",
	})
	st, _ := ns.UpsertStation(context.Background(), network.StationInput{Name: "S1", Code: "C1", Type: network.StationCompressor})
	ns.UpsertStation(context.Background(), network.StationInput{Name: "S2", Code: "C2", Type: network.StationDelivery})
	cmp, _ := ns.UpsertCompressor(context.Background(), network.CompressorInput{
		Name: "CMP1", StationID: st.ID, SegmentID: seg.ID, Model: "X", PowerMW: 10, State: network.CompressorStopped,
	})
	v, _ := ns.UpsertValve(context.Background(), network.ValveInput{
		Name: "V1", SegmentID: seg.ID, Type: "block", State: network.ValveOpen, Normal: "open",
	})
	_ = cmp
	_ = v
	return &env{dispatch: ds, network: ns, clock: c}
}

func TestDispatchCreateInvalidTarget(t *testing.T) {
	e := newEnv(t)
	if _, err := e.dispatch.Create(context.Background(), Input{
		Type: TypeStartCompressor, TargetID: "nope", Reason: "r",
	}); err == nil {
		t.Fatalf("unknown compressor should fail")
	}
}

func TestDispatchFullFlowCompressor(t *testing.T) {
	e := newEnv(t)
	cs := e.network.ListCompressors(context.Background())
	if len(cs) == 0 {
		t.Fatal("no compressor seeded")
	}
	cmp := cs[0]
	o, err := e.dispatch.Create(context.Background(), Input{
		Type: TypeStartCompressor, TargetID: cmp.ID, Reason: "boost",
	})
	if err != nil {
		t.Fatal(err)
	}
	if o.State != StatePending {
		t.Fatalf("state = %s", o.State)
	}
	o, err = e.dispatch.Issue(context.Background(), o.ID, "dispatch")
	if err != nil || o.State != StateIssued {
		t.Fatalf("issue: %v %s", err, o.State)
	}
	o, err = e.dispatch.Execute(context.Background(), o.ID)
	if err != nil || o.State != StateExecuted {
		t.Fatalf("execute: %v %s", err, o.State)
	}
	// compressor should now be running
	cmp, _ = e.network.GetCompressor(context.Background(), cmp.ID)
	if cmp.State != network.CompressorRunning {
		t.Fatalf("compressor state = %s, want running", cmp.State)
	}
}

func TestDispatchIssueRejectsNoOp(t *testing.T) {
	e := newEnv(t)
	cs := e.network.ListCompressors(context.Background())
	cmp := cs[0]
	// start the compressor first so a start order is a no-op
	e.network.ChangeCompressorState(context.Background(), cmp.ID, network.CompressorRunning, "pre")
	o, _ := e.dispatch.Create(context.Background(), Input{
		Type: TypeStartCompressor, TargetID: cmp.ID, Reason: "x",
	})
	if _, err := e.dispatch.Issue(context.Background(), o.ID, "d"); err == nil {
		t.Fatalf("issuing start on running compressor should fail")
	}
}

func TestDispatchRevoke(t *testing.T) {
	e := newEnv(t)
	cs := e.network.ListCompressors(context.Background())
	cmp := cs[0]
	o, _ := e.dispatch.Create(context.Background(), Input{
		Type: TypeStartCompressor, TargetID: cmp.ID, Reason: "x",
	})
	o, err := e.dispatch.Revoke(context.Background(), o.ID)
	if err != nil || o.State != StateRevoked {
		t.Fatalf("revoke: %v %s", err, o.State)
	}
	// cannot execute a revoked order
	if _, err := e.dispatch.Execute(context.Background(), o.ID); err == nil {
		t.Fatalf("execute after revoke should fail")
	}
}

func TestDispatchDoubleExecute(t *testing.T) {
	e := newEnv(t)
	cs := e.network.ListCompressors(context.Background())
	cmp := cs[0]
	o, _ := e.dispatch.Create(context.Background(), Input{
		Type: TypeStopCompressor, TargetID: cmp.ID, Reason: "x",
	})
	e.network.ChangeCompressorState(context.Background(), cmp.ID, network.CompressorRunning, "pre")
	e.dispatch.Issue(context.Background(), o.ID, "d")
	if _, err := e.dispatch.Execute(context.Background(), o.ID); err != nil {
		t.Fatalf("first execute: %v", err)
	}
	if _, err := e.dispatch.Execute(context.Background(), o.ID); err == nil {
		t.Fatalf("double execute should fail")
	}
}

func _useTime() time.Time { return time.Now() }
