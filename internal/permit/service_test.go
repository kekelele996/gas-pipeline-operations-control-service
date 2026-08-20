package permit

import (
	"context"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
)

func newSvc(t *testing.T) (*Service, *platform.FakeClock) {
	t.Helper()
	c := platform.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	return NewService(NewStore(), c, nil, nil, nil), c
}

func baseInput() Input {
	w := time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC)
	return Input{
		SegmentID: "SEG1", WorkType: "pigging", Title: "Pig run", Applicant: "alice",
		WindowStart: w, WindowEnd: w.Add(8 * time.Hour),
	}
}

func TestPermitFullFlow(t *testing.T) {
	s, _ := newSvc(t)
	p, err := s.Apply(context.Background(), baseInput())
	if err != nil {
		t.Fatal(err)
	}
	if p.State != StatePending {
		t.Fatalf("state = %s", p.State)
	}
	p, err = s.Approve(context.Background(), p.ID, "bob")
	if err != nil || p.State != StateApproved {
		t.Fatalf("approve: %v %s", err, p.State)
	}
	p, err = s.Start(context.Background(), p.ID)
	if err != nil || p.State != StateInProgress {
		t.Fatalf("start: %v %s", err, p.State)
	}
	p, err = s.Complete(context.Background(), p.ID)
	if err != nil || p.State != StateCompleted {
		t.Fatalf("complete: %v %s", err, p.State)
	}
}

func TestPermitConflictOnSameSegment(t *testing.T) {
	s, _ := newSvc(t)
	p1, _ := s.Apply(context.Background(), baseInput())
	if _, err := s.Approve(context.Background(), p1.ID, "bob"); err != nil {
		t.Fatalf("approve p1: %v", err)
	}
	// second overlapping permit on the same segment
	p2, _ := s.Apply(context.Background(), baseInput())
	_, err := s.Approve(context.Background(), p2.ID, "bob")
	if err == nil {
		t.Fatalf("overlapping permit should be rejected")
	}
}

type fakeDispatchChecker struct{ ids []string }

func (f fakeDispatchChecker) PendingOrdersForSegment(string) []string { return f.ids }

type fakeIncidentChecker struct{ ids []string }

func (f fakeIncidentChecker) OpenIncidentsForSegment(string) []string { return f.ids }

func TestPermitConflictOnDispatchOrder(t *testing.T) {
	c := platform.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	s := NewService(NewStore(), c, fakeDispatchChecker{[]string{"ORD1"}}, nil, nil)
	p, _ := s.Apply(context.Background(), baseInput())
	if _, err := s.Approve(context.Background(), p.ID, "bob"); err == nil {
		t.Fatalf("pending dispatch order should block approval")
	}
}

func TestPermitConflictOnIncident(t *testing.T) {
	c := platform.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	s := NewService(NewStore(), c, nil, fakeIncidentChecker{[]string{"INC1"}}, nil)
	p, _ := s.Apply(context.Background(), baseInput())
	if _, err := s.Approve(context.Background(), p.ID, "bob"); err == nil {
		t.Fatalf("open incident should block approval")
	}
}

func TestPermitExpireScan(t *testing.T) {
	s, clock := newSvc(t)
	p, _ := s.Apply(context.Background(), baseInput())
	s.Approve(context.Background(), p.ID, "bob")
	// advance past the window end
	clock.Advance(72 * time.Hour)
	expired := s.ExpireScan(context.Background())
	if len(expired) != 1 || expired[0] != p.ID {
		t.Fatalf("expected p expired, got %v", expired)
	}
	got, _ := s.Get(context.Background(), p.ID)
	if got.State != StateExpired {
		t.Fatalf("state = %s, want expired", got.State)
	}
}

func TestPermitInvalidTransition(t *testing.T) {
	s, _ := newSvc(t)
	p, _ := s.Apply(context.Background(), baseInput())
	// cannot start a pending permit (must approve first)
	if _, err := s.Start(context.Background(), p.ID); err == nil {
		t.Fatalf("start from pending should fail")
	}
}
