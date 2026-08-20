package permit

import (
	"context"
	"errors"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/audit"
	"gas-pipeline-operations-control-service/internal/platform"
)

func newSvcR007(t *testing.T) *Service {
	t.Helper()
	auditSvc := audit.NewService(audit.NewStore(100), platform.SystemClock{})
	return NewService(NewStore(), platform.SystemClock{}, nil, nil, auditSvc)
}

func applyPermitR007(t *testing.T, svc *Service, id, seg string, start, end time.Time) Permit {
	t.Helper()
	in := Input{
		SegmentID: seg, WorkType: "inspection", Title: "inspect", Applicant: "ops",
		WindowStart: start, WindowEnd: end,
	}
	p, err := svc.Apply(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestPermitApproveConflictNotSwallowedR007A: approving an overlapping permit
// must return the conflict error and leave the permit pending.
func TestPermitApproveConflictNotSwallowedR007A(t *testing.T) {
	svc := newSvcR007(t)
	base := time.Now()
	a := applyPermitR007(t, svc, "", "SEG-1", base, base.Add(2*time.Hour))
	if _, err := svc.Approve(context.Background(), a.ID, "mgr"); err != nil {
		t.Fatal(err)
	}
	b := applyPermitR007(t, svc, "", "SEG-1", base.Add(time.Hour), base.Add(3*time.Hour))
	_, err := svc.Approve(context.Background(), b.ID, "mgr")
	if err == nil {
		t.Fatal("overlapping approval returned no error")
	}
	if !errors.Is(err, platform.ErrConflict) {
		t.Fatalf("overlapping approval error = %v, want conflict", err)
	}
	got, _ := svc.Get(context.Background(), b.ID)
	if got.State != StatePending {
		t.Fatalf("conflicting permit state = %s, want pending", got.State)
	}
}

// TestPermitApproveIllegalStateR007B: double-approving must return the state
// error, not a silent success.
func TestPermitApproveIllegalStateR007B(t *testing.T) {
	svc := newSvcR007(t)
	base := time.Now()
	p := applyPermitR007(t, svc, "", "SEG-2", base, base.Add(2*time.Hour))
	if _, err := svc.Approve(context.Background(), p.ID, "mgr"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Approve(context.Background(), p.ID, "mgr"); err == nil {
		t.Fatal("double approve returned no error")
	}
}

// TestPermitCompleteErrorPreservedR007C: completing a non-in-progress permit
// must return the state error.
func TestPermitCompleteErrorPreservedR007C(t *testing.T) {
	svc := newSvcR007(t)
	base := time.Now()
	p := applyPermitR007(t, svc, "", "SEG-3", base, base.Add(2*time.Hour))
	if _, err := svc.Complete(context.Background(), p.ID); err == nil {
		t.Fatal("completing a pending permit returned no error")
	}
}

// TestPermitCancelErrorPreservedR007D: cancelling an already-completed permit
// must return the state error.
func TestPermitCancelErrorPreservedR007D(t *testing.T) {
	svc := newSvcR007(t)
	base := time.Now()
	p := applyPermitR007(t, svc, "", "SEG-4", base, base.Add(2*time.Hour))
	if _, err := svc.Approve(context.Background(), p.ID, "mgr"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Start(context.Background(), p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Complete(context.Background(), p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Cancel(context.Background(), p.ID, "done"); err == nil {
		t.Fatal("cancelling a completed permit returned no error")
	}
}

// TestPermitStartErrorPreservedR007E: starting a non-approved permit must
// return the state error.
func TestPermitStartErrorPreservedR007E(t *testing.T) {
	svc := newSvcR007(t)
	base := time.Now()
	p := applyPermitR007(t, svc, "", "SEG-5", base, base.Add(2*time.Hour))
	if _, err := svc.Start(context.Background(), p.ID); err == nil {
		t.Fatal("starting a pending permit returned no error")
	}
}

// TestPermitNonOverlappingApproveR007G: non-overlapping permits on the same
// segment must both be approvable.
func TestPermitNonOverlappingApproveR007G(t *testing.T) {
	svc := newSvcR007(t)
	base := time.Now()
	a := applyPermitR007(t, svc, "", "SEG-6", base, base.Add(2*time.Hour))
	b := applyPermitR007(t, svc, "", "SEG-6", base.Add(3*time.Hour), base.Add(5*time.Hour))
	if _, err := svc.Approve(context.Background(), a.ID, "mgr"); err != nil {
		t.Fatal(err)
	}
	approved, err := svc.Approve(context.Background(), b.ID, "mgr")
	if err != nil {
		t.Fatalf("non-overlapping approval failed: %v", err)
	}
	if approved.State != StateApproved {
		t.Fatalf("b state = %s, want approved", approved.State)
	}
}
