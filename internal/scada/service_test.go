package scada

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
)

func newTestService() (*Service, *platform.FakeClock) {
	c := platform.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	s := NewService(NewStore(50), c, 0.5)
	return s, c
}

func mustPoint(s *Service, id string) Point {
	return s.RegisterPoint(context.Background(), Point{
		ID: id, Name: id, Type: TypePressure, Unit: "MPa",
		HighLimit: 9.0, LowLimit: 0.5, RateLimit: 1.0, Enabled: true,
		PhysicalMin: -0.1, PhysicalMax: 20.0,
	})
}

func TestIngestFiltersBadValues(t *testing.T) {
	s, _ := newTestService()
	mustPoint(s, "P1")
	res := s.Ingest(context.Background(), []Reading{
		{PointID: "P1", Value: math.NaN()},
		{PointID: "P1", Value: math.Inf(1)},
		{PointID: "P1", Value: 999.0}, // outside physical max
		{PointID: "P1", Value: 5.0},
	})
	if res[0].Stored || res[0].Rejected == "" {
		t.Fatalf("NaN should be rejected: %+v", res[0])
	}
	if res[1].Stored || res[1].Rejected == "" {
		t.Fatalf("Inf should be rejected: %+v", res[1])
	}
	if res[2].Stored || res[2].Rejected == "" {
		t.Fatalf("out-of-range should be rejected: %+v", res[2])
	}
	if !res[3].Stored {
		t.Fatalf("good value should be stored: %+v", res[3])
	}
	hist, _ := s.History(context.Background(), "P1", 0)
	if len(hist) != 1 {
		t.Fatalf("expected 1 stored reading, got %d", len(hist))
	}
}

func TestThresholdAlarm(t *testing.T) {
	s, _ := newTestService()
	mustPoint(s, "P1")
	s.Ingest(context.Background(), []Reading{
		{PointID: "P1", Value: 5.0, Ts: time.Now()},
		{PointID: "P1", Value: 9.5, Ts: time.Now()}, // above high limit
	})
	alarms := s.Alarms(context.Background(), "P1", false)
	if len(alarms) != 1 {
		t.Fatalf("expected 1 alarm, got %d", len(alarms))
	}
	if alarms[0].Level != AlarmHigh {
		t.Fatalf("expected high alarm, got %s", alarms[0].Level)
	}
}

func TestRateAlarm(t *testing.T) {
	s, clock := newTestService()
	p := mustPoint(s, "P1")
	// explicit rate rule: 0.1 MPa/s
	s.AddRule(context.Background(), Rule{
		PointID: p.ID, Kind: RuleRate, Level: AlarmCritical,
		Message: "too fast", Rate: 0.1, Enabled: true,
	})
	t0 := clock.Now()
	s.Ingest(context.Background(), []Reading{
		{PointID: "P1", Value: 5.0, Ts: t0},
		{PointID: "P1", Value: 7.0, Ts: t0.Add(time.Second)}, // 2 MPa/s > 0.1
	})
	alarms := s.Alarms(context.Background(), "P1", false)
	if len(alarms) == 0 {
		t.Fatalf("expected rate alarm")
	}
}

func TestAlarmStateTransitions(t *testing.T) {
	s, _ := newTestService()
	mustPoint(s, "P1")
	s.Ingest(context.Background(), []Reading{
		{PointID: "P1", Value: 9.5, Ts: time.Now()},
	})
	alarms := s.Alarms(context.Background(), "P1", false)
	if len(alarms) != 1 {
		t.Fatalf("expected 1 alarm, got %d", len(alarms))
	}
	id := alarms[0].ID
	if _, err := s.AckAlarm(context.Background(), id, "op"); err != nil {
		t.Fatalf("ack: %v", err)
	}
	if _, err := s.ResolveAlarm(context.Background(), id); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	// already resolved -> illegal
	if _, err := s.AckAlarm(context.Background(), id, "op"); err == nil {
		t.Fatalf("ack after resolve should fail")
	}
}

// TestScadaConcurrentIngestSnapshotStable runs many goroutines ingesting into
// the same point and verifies the ring buffer never panics under the race
// detector and that a captured snapshot is not mutated by later writes.
func TestScadaConcurrentIngestSnapshotStable(t *testing.T) {
	s, _ := newTestService()
	mustPoint(s, "P1")
	const goroutines = 16
	const perG = 200
	var wg sync.WaitGroup
	start := make(chan struct{})
	var stored int64
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start
			for i := 0; i < perG; i++ {
				res := s.Ingest(context.Background(), []Reading{
					{PointID: "P1", Value: float64(g*1000 + i%10), Ts: time.Now()},
				})
				if res[0].Stored {
					atomic.AddInt64(&stored, 1)
				}
			}
		}(g)
	}
	close(start)
	wg.Wait()

	// take a snapshot, then keep ingesting, and confirm the snapshot is stable
	snap := s.Store().History("P1")
	if len(snap) == 0 {
		t.Fatalf("snapshot empty")
	}
	// copy the values
	firstVals := make([]float64, len(snap))
	for i, r := range snap {
		firstVals[i] = r.Value
	}
	// more writes
	for i := 0; i < 30; i++ {
		s.Ingest(context.Background(), []Reading{{PointID: "P1", Value: float64(i), Ts: time.Now()}})
	}
	snap2 := s.Store().History("P1")
	if len(snap2) > 50 {
		t.Fatalf("ring buffer over capacity: %d", len(snap2))
	}
	for i, v := range firstVals {
		if snap[i].Value != v {
			t.Fatalf("snapshot mutated at %d: %g != %g", i, snap[i].Value, v)
		}
	}
}

// TestHistoryDeepCopy confirms the returned slice is independent of the
// internal buffer: appending to it must not affect subsequent reads.
func TestHistoryDeepCopy(t *testing.T) {
	s, _ := newTestService()
	mustPoint(s, "P1")
	s.Ingest(context.Background(), []Reading{
		{PointID: "P1", Value: 1.0, Ts: time.Now()},
		{PointID: "P1", Value: 2.0, Ts: time.Now()},
	})
	hist, _ := s.History(context.Background(), "P1", 0)
	hist[0].Value = 999 // mutate the returned copy
	hist2, _ := s.History(context.Background(), "P1", 0)
	if hist2[0].Value == 999 {
		t.Fatalf("internal buffer leaked via returned slice")
	}
}

func TestStatsCounters(t *testing.T) {
	s, _ := newTestService()
	mustPoint(s, "P1")
	s.Ingest(context.Background(), []Reading{
		{PointID: "P1", Value: 5.0, Ts: time.Now()},
		{PointID: "P1", Value: 9.5, Ts: time.Now()}, // alarm
	})
	pnt, rds, alm, act := s.Stats(context.Background())
	if pnt != 1 || rds != 2 || alm != 1 || act != 1 {
		t.Fatalf("stats wrong: p=%d r=%d a=%d act=%d", pnt, rds, alm, act)
	}
}

func TestRingBufferEvictionOrder(t *testing.T) {
	s, _ := newTestService()
	mustPoint(s, "P1")
	for i := 0; i < 60; i++ {
		s.Ingest(context.Background(), []Reading{
			{PointID: "P1", Value: float64(i) * 0.1, Ts: time.Now().Add(time.Duration(i) * time.Second)},
		})
	}
	hist, _ := s.History(context.Background(), "P1", 0)
	if len(hist) != 50 {
		t.Fatalf("expected 50 after eviction, got %d", len(hist))
	}
	// oldest should be reading #10 (value 1.0)
	if hist[0].Value != 1.0 {
		t.Fatalf("expected oldest=1.0, got %g", hist[0].Value)
	}
	// newest should be 5.9
	if hist[len(hist)-1].Value != 5.9 {
		t.Fatalf("expected newest=5.9, got %g", hist[len(hist)-1].Value)
	}
}

func TestDisabledPointRejected(t *testing.T) {
	s, _ := newTestService()
	p := mustPoint(s, "P1")
	p.Enabled = false
	s.RegisterPoint(context.Background(), p)
	res := s.Ingest(context.Background(), []Reading{{PointID: "P1", Value: 5.0}})
	if res[0].Stored {
		t.Fatalf("disabled point should reject")
	}
	if res[0].Rejected == "" {
		t.Fatalf("expected rejection reason")
	}
}

// ensure the format import is used when alarms carry formatted messages
var _ = fmt.Sprintf
