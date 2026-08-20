package incident

import (
	"context"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
	"gas-pipeline-operations-control-service/internal/scada"
)

type fakeAlarmResolver struct{ resolved []string }

func (f *fakeAlarmResolver) ResolveAlarm(ctx context.Context, id string) error {
	f.resolved = append(f.resolved, id)
	return nil
}

func newSvc(t *testing.T) (*Service, *platform.FakeClock, *fakeAlarmResolver) {
	t.Helper()
	c := platform.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	r := &fakeAlarmResolver{}
	return NewService(NewStore(), c, nil, r), c, r
}

func TestIncidentFullFlow(t *testing.T) {
	s, _, r := newSvc(t)
	i, err := s.Report(context.Background(), Input{
		SegmentID: "SEG1", Severity: SeveritySerious, Title: "pressure spike",
		Description: "overpressure", Reporter: "alice", AlarmIDs: []string{"ALM1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if i.State != StatePending {
		t.Fatalf("state = %s", i.State)
	}
	if _, err := s.Confirm(context.Background(), i.ID, "bob"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := s.AddAction(context.Background(), i.ID, "inspect", "carol"); err != nil {
		t.Fatalf("add action: %v", err)
	}
	// cannot close with incomplete actions
	if _, err := s.Close(context.Background(), i.ID); err == nil {
		t.Fatalf("close with incomplete actions should fail")
	}
	actions := s.List(context.Background())[0].Actions
	if _, err := s.CompleteAction(context.Background(), i.ID, actions[0].ID); err != nil {
		t.Fatalf("complete action: %v", err)
	}
	if _, err := s.Close(context.Background(), i.ID); err != nil {
		t.Fatalf("close: %v", err)
	}
	// alarm should have been resolved on close
	if len(r.resolved) != 1 || r.resolved[0] != "ALM1" {
		t.Fatalf("alarm not resolved: %v", r.resolved)
	}
}

func TestIncidentConfirmFromHandlingFails(t *testing.T) {
	s, _, _ := newSvc(t)
	i, _ := s.Report(context.Background(), Input{
		SegmentID: "SEG1", Severity: SeverityWarning, Title: "x", Reporter: "a",
	})
	s.Confirm(context.Background(), i.ID, "b")
	if _, err := s.Confirm(context.Background(), i.ID, "c"); err == nil {
		t.Fatalf("confirm from handling should fail")
	}
}

func TestIncidentCannotCloseWithoutActions(t *testing.T) {
	s, _, _ := newSvc(t)
	i, _ := s.Report(context.Background(), Input{
		SegmentID: "SEG1", Severity: SeverityInfo, Title: "x", Reporter: "a",
	})
	s.Confirm(context.Background(), i.ID, "b")
	if _, err := s.Close(context.Background(), i.ID); err == nil {
		t.Fatalf("close with no actions should fail")
	}
}

func TestIncidentOpenIncidentsForSegment(t *testing.T) {
	s, _, _ := newSvc(t)
	s.Report(context.Background(), Input{SegmentID: "SEG1", Severity: "warning", Title: "a", Reporter: "x"})
	s.Report(context.Background(), Input{SegmentID: "SEG2", Severity: "warning", Title: "b", Reporter: "x"})
	if got := len(s.OpenIncidentsForSegment("SEG1")); got != 1 {
		t.Fatalf("open for SEG1 = %d", got)
	}
}

// keep scada import referenced (alarm resolver interface shape mirrors it)
var _ scada.AlarmLevel
