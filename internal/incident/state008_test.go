package incident

import (
	"context"
	"testing"

	"gas-pipeline-operations-control-service/internal/audit"
	"gas-pipeline-operations-control-service/internal/platform"
)

func newSvcR008(t *testing.T) *Service {
	t.Helper()
	auditSvc := audit.NewService(audit.NewStore(100), platform.SystemClock{})
	return NewService(NewStore(), platform.SystemClock{}, auditSvc, nil)
}

func reportR008(t *testing.T, svc *Service, seg string) Incident {
	t.Helper()
	i, err := svc.Report(context.Background(), Input{
		SegmentID: seg, Severity: SeverityWarning, Title: "pressure drop",
		Reporter: "ops",
	})
	if err != nil {
		t.Fatal(err)
	}
	return i
}

func openR008(t *testing.T, svc *Service, i Incident) Incident {
	t.Helper()
	o, err := svc.Confirm(context.Background(), i.ID, "ops")
	if err != nil {
		t.Fatal(err)
	}
	return o
}

// TestIncidentEscalateThenCloseR008A: an escalated incident must be able to
// close once its actions are complete.
func TestIncidentEscalateThenCloseR008A(t *testing.T) {
	svc := newSvcR008(t)
	i := openR008(t, svc, reportR008(t, svc, "SEG-1"))
	withActions, err := svc.AddAction(context.Background(), i.ID, "fix it", "ops")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CompleteAction(context.Background(), withActions.ID, withActions.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	esc, err := svc.Escalate(context.Background(), i.ID, "needs extra staff")
	if err != nil {
		t.Fatal(err)
	}
	_ = esc
	closed, err := svc.Close(context.Background(), i.ID)
	if err != nil {
		t.Fatalf("close after escalate: %v", err)
	}
	if closed.State != StateClosed {
		t.Fatalf("incident state = %s, want closed", closed.State)
	}
}

// TestIncidentEscalatedCountedOpenR008B: escalated incidents must appear in
// the open list.
func TestIncidentEscalatedCountedOpenR008B(t *testing.T) {
	svc := newSvcR008(t)
	i := openR008(t, svc, reportR008(t, svc, "SEG-2"))
	if _, err := svc.Escalate(context.Background(), i.ID, "escalate"); err != nil {
		t.Fatal(err)
	}
	open := svc.ListOpen(context.Background())
	found := false
	for _, o := range open {
		if o.ID == i.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("escalated incident missing from open list")
	}
}

// TestIncidentEscalateStaleWriteR008C: Escalate must return the incident in
// its escalated state.
func TestIncidentEscalateStaleWriteR008C(t *testing.T) {
	svc := newSvcR008(t)
	i := openR008(t, svc, reportR008(t, svc, "SEG-3"))
	esc, err := svc.Escalate(context.Background(), i.ID, "escalate")
	if err != nil {
		t.Fatal(err)
	}
	if esc.State != StateEscalated {
		t.Fatalf("Escalate returned state %s, want escalated", esc.State)
	}
}

// TestIncidentEscalatedBlocksPermitR008D: an escalated incident must still be
// seen as open by the permit conflict checker.
func TestIncidentEscalatedBlocksPermitR008D(t *testing.T) {
	svc := newSvcR008(t)
	i := openR008(t, svc, reportR008(t, svc, "SEG-4"))
	if _, err := svc.Escalate(context.Background(), i.ID, "escalate"); err != nil {
		t.Fatal(err)
	}
	ids := svc.OpenIncidentsForSegment("SEG-4")
	found := false
	for _, id := range ids {
		if id == i.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("escalated incident not returned by OpenIncidentsForSegment")
	}
}

// TestIncidentCloseRequiresActionsR008E: closing an incident with no action
// items must be rejected.
func TestIncidentCloseRequiresActionsR008E(t *testing.T) {
	svc := newSvcR008(t)
	i := openR008(t, svc, reportR008(t, svc, "SEG-5"))
	if _, err := svc.Close(context.Background(), i.ID); err == nil {
		t.Fatal("closed an incident without action items")
	}
}
