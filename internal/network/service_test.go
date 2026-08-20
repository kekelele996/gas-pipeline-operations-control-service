package network

import (
	"context"
	"sync"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
)

func newSvc(t *testing.T) (*Service, *platform.FakeClock) {
	t.Helper()
	c := platform.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	return NewService(NewStore(), c, nil), c
}

func mustSegment(s *Service) Segment {
	seg, err := s.UpsertSegment(context.Background(), SegmentInput{
		Name: "Trunk A", From: "STN1", To: "STN2",
		LengthKm: 100, DiameterMm: 1016, MAOPMPa: 9.0, Tier: "main", Material: "X70",
	})
	if err != nil {
		panic(err)
	}
	return seg
}

func TestUpsertSegmentValidates(t *testing.T) {
	s, _ := newSvc(t)
	if _, err := s.UpsertSegment(context.Background(), SegmentInput{Name: "", From: "a", To: "b"}); err == nil {
		t.Fatalf("empty name should fail")
	}
	if _, err := s.UpsertSegment(context.Background(), SegmentInput{
		Name: "X", From: "a", To: "a", LengthKm: 1, DiameterMm: 1, MAOPMPa: 1, Tier: "main",
	}); err == nil {
		t.Fatalf("from==to should fail")
	}
}

func TestCompressorStateMachine(t *testing.T) {
	s, _ := newSvc(t)
	seg := mustSegment(s)
	st, err := s.UpsertStation(context.Background(), StationInput{Name: "STN1", Code: "S1", Type: StationCompressor})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.UpsertCompressor(context.Background(), CompressorInput{
		Name: "CMP1", StationID: st.ID, SegmentID: seg.ID, Model: "X", PowerMW: 20, State: CompressorStopped,
	})
	if err != nil {
		t.Fatal(err)
	}
	// stopped -> running ok
	c, err = s.ChangeCompressorState(context.Background(), c.ID, CompressorRunning, "test")
	if err != nil || c.State != CompressorRunning {
		t.Fatalf("stop->run: %v %s", err, c.State)
	}
	// running -> running illegal
	if _, err := s.ChangeCompressorState(context.Background(), c.ID, CompressorRunning, ""); err == nil {
		t.Fatalf("running->running should fail")
	}
	// running -> maintenance ok
	c, err = s.ChangeCompressorState(context.Background(), c.ID, CompressorMaintenance, "maint")
	if err != nil || c.State != CompressorMaintenance {
		t.Fatalf("run->maint: %v %s", err, c.State)
	}
}

func TestValveStateMachine(t *testing.T) {
	s, _ := newSvc(t)
	seg := mustSegment(s)
	v, err := s.UpsertValve(context.Background(), ValveInput{
		Name: "V1", SegmentID: seg.ID, Type: "block", State: ValveClosed, Normal: "closed",
	})
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.ChangeValveState(context.Background(), v.ID, ValveOpen)
	if err != nil || v.State != ValveOpen {
		t.Fatalf("closed->open: %v %s", err, v.State)
	}
	if _, err := s.ChangeValveState(context.Background(), v.ID, ValveClosed); err != nil {
		t.Fatalf("open->closed: %v", err)
	}
	// closed -> closed illegal
	if _, err := s.ChangeValveState(context.Background(), v.ID, ValveClosed); err == nil {
		t.Fatalf("closed->closed should fail")
	}
}

func TestStoreReturnsCopies(t *testing.T) {
	s, _ := newSvc(t)
	seg := mustSegment(s)
	got, _ := s.store.Segment(seg.ID)
	got.Name = "MUTATED"
	got2, _ := s.store.Segment(seg.ID)
	if got2.Name == "MUTATED" {
		t.Fatalf("store leaked internal pointer")
	}
}

func TestConcurrentDeviceStateChange(t *testing.T) {
	s, _ := newSvc(t)
	seg := mustSegment(s)
	st, err := s.UpsertStation(context.Background(), StationInput{Name: "STN1", Code: "S1", Type: StationCompressor})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := s.UpsertCompressor(context.Background(), CompressorInput{
		Name: "CMP1", StationID: st.ID, SegmentID: seg.ID, Model: "X", PowerMW: 20, State: CompressorStopped,
	})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// alternate legal-ish attempts; only some should succeed
			target := CompressorRunning
			if i%2 == 0 {
				target = CompressorMaintenance
			}
			s.ChangeCompressorState(context.Background(), c.ID, target, "")
		}(i)
	}
	wg.Wait()
	// must end in a known state without panicking
	out, _ := s.store.Compressor(c.ID)
	if !ValidCompressorState(out.State.String()) {
		t.Fatalf("invalid final state %s", out.State)
	}
}
