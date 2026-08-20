package contract

import (
	"context"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
)

func newSvc() (*Service, *platform.FakeClock) {
	c := platform.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	return NewService(NewStore(), c, nil), c
}

func mustCreate(s *Service) Contract {
	c, _ := s.Create(context.Background(), ContractInput{
		ShipperID: "S1", ShipperName: "Shipper", Code: "C1",
		DailyVolume: 1000, ValidFrom: s.clock.Now(), ValidTo: s.clock.Now().Add(365 * 24 * time.Hour),
	})
	return c
}

func TestCreateValidates(t *testing.T) {
	s, _ := newSvc()
	if _, err := s.Create(context.Background(), ContractInput{ShipperID: "", DailyVolume: 100}); err == nil {
		t.Fatalf("empty shipper should fail")
	}
	if _, err := s.Create(context.Background(), ContractInput{ShipperID: "S", ShipperName: "N", Code: "C", DailyVolume: -1}); err == nil {
		t.Fatalf("negative volume should fail")
	}
}

func TestReserveAndRemaining(t *testing.T) {
	s, _ := newSvc()
	c := mustCreate(s)
	rem, _ := s.Remaining(context.Background(), c.ID)
	if rem != 1000 {
		t.Fatalf("remaining = %g", rem)
	}
	s.Reserve(context.Background(), c.ID, 300)
	rem, _ = s.Remaining(context.Background(), c.ID)
	if rem != 700 {
		t.Fatalf("after reserve remaining = %g, want 700", rem)
	}
}

func TestReserveRejectsOversell(t *testing.T) {
	s, _ := newSvc()
	c := mustCreate(s)
	if _, err := s.Reserve(context.Background(), c.ID, 1001); err == nil {
		t.Fatalf("oversell should be rejected")
	}
}

func TestReleaseNeverNegative(t *testing.T) {
	s, _ := newSvc()
	c := mustCreate(s)
	s.Reserve(context.Background(), c.ID, 200)
	s.Release(context.Background(), c.ID, 500) // over-release
	out, _ := s.Get(context.Background(), c.ID)
	if out.UsedVolume != 0 {
		t.Fatalf("used = %g, want 0", out.UsedVolume)
	}
}

func TestSetStateSuspendActivate(t *testing.T) {
	s, _ := newSvc()
	c := mustCreate(s)
	c, _ = s.SetState(context.Background(), c.ID, StateSuspended)
	if c.State != StateSuspended {
		t.Fatalf("state = %s", c.State)
	}
	// cannot reserve on suspended
	if _, err := s.Reserve(context.Background(), c.ID, 100); err == nil {
		t.Fatalf("reserve on suspended should fail")
	}
	c, _ = s.SetState(context.Background(), c.ID, StateActive)
	if c.State != StateActive {
		t.Fatalf("state = %s", c.State)
	}
}

func TestExpireDue(t *testing.T) {
	s, clock := newSvc()
	// contract already expired
	c, _ := s.Create(context.Background(), ContractInput{
		ShipperID: "S", ShipperName: "N", Code: "C", DailyVolume: 100,
		ValidFrom: clock.Now().Add(-365 * 24 * time.Hour),
		ValidTo:   clock.Now().Add(-1 * time.Hour),
	})
	clock.Advance(2 * time.Hour) // now past ValidTo
	expired := s.ExpireDue(context.Background())
	if len(expired) != 1 || expired[0] != c.ID {
		t.Fatalf("expected c expired, got %v", expired)
	}
}
