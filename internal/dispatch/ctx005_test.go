package dispatch

import (
	"context"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/audit"
	"gas-pipeline-operations-control-service/internal/network"
	"gas-pipeline-operations-control-service/internal/platform"
)

// fakeNetwork implements NetworkService and observes cancellation: every
// device call returns ctx.Err() when the context is done, and Change* calls
// block until the context completes or a short window elapses.
type fakeNetwork struct {
	block time.Duration
}

func (f *fakeNetwork) GetCompressor(ctx context.Context, id string) (network.Compressor, error) {
	if err := ctx.Err(); err != nil {
		return network.Compressor{}, err
	}
	return network.Compressor{ID: id, SegmentID: "SEG-1", State: network.CompressorStopped}, nil
}

func (f *fakeNetwork) GetValve(ctx context.Context, id string) (network.Valve, error) {
	if err := ctx.Err(); err != nil {
		return network.Valve{}, err
	}
	return network.Valve{ID: id, SegmentID: "SEG-1", State: network.ValveClosed}, nil
}

func (f *fakeNetwork) ChangeCompressorState(ctx context.Context, id string, target network.CompressorState, reason string) (network.Compressor, error) {
	out := network.Compressor{ID: id, SegmentID: "SEG-1", State: target}
	if err := f.wait(ctx); err != nil {
		return out, err
	}
	return out, nil
}

func (f *fakeNetwork) ChangeValveState(ctx context.Context, id string, target network.ValveState) (network.Valve, error) {
	out := network.Valve{ID: id, SegmentID: "SEG-1", State: target}
	if err := f.wait(ctx); err != nil {
		return out, err
	}
	return out, nil
}

func (f *fakeNetwork) wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(f.block):
		return nil
	}
}

func newSvcR005(t *testing.T, f *fakeNetwork) *Service {
	t.Helper()
	auditSvc := audit.NewService(audit.NewStore(100), platform.SystemClock{})
	if f == nil {
		f = &fakeNetwork{}
	}
	return NewService(NewStore(), f, platform.SystemClock{}, auditSvc)
}

// TestDispatchCreateCancelR005C: creating an order with a cancelled context
// must not silently proceed against a live device lookup.
func TestDispatchCreateCancelR005C(t *testing.T) {
	svc := newSvcR005(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := svc.Create(ctx, Input{Type: TypeOpenValve, TargetID: "V-1", Reason: "test"})
	if err == nil {
		t.Fatal("create succeeded on cancelled context")
	}
}

// TestDispatchIssueCancelR005B: issuing an order with a cancelled context
// must fail.
func TestDispatchIssueCancelR005B(t *testing.T) {
	svc := newSvcR005(t, nil)
	o, err := svc.Create(context.Background(), Input{Type: TypeOpenValve, TargetID: "V-1", Reason: "test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.Issue(ctx, o.ID, "ops"); err == nil {
		t.Fatal("issue succeeded on cancelled context")
	}
}

// TestDispatchExecuteCancelR005A: executing an order whose context times out
// mid-operation must fail and must not complete the device change.
func TestDispatchExecuteCancelR005A(t *testing.T) {
	svc := newSvcR005(t, &fakeNetwork{block: 200 * time.Millisecond})
	o, err := svc.Create(context.Background(), Input{Type: TypeOpenValve, TargetID: "V-1", Reason: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Issue(context.Background(), o.ID, "ops"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := svc.Execute(ctx, o.ID); err == nil {
		t.Fatal("execute completed despite context timeout")
	}
	// the order must not be marked executed
	got, _ := svc.Get(context.Background(), o.ID)
	if got.State == StateExecuted {
		t.Fatal("order marked executed after cancelled device op")
	}
}
