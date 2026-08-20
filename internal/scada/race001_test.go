package scada

import (
	"context"
	"sync"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
)

func newRace001Svc(t *testing.T, bufCap int) *Service {
	t.Helper()
	st := NewStore(bufCap)
	svc := NewService(st, platform.SystemClock{}, 0.5)
	svc.RegisterPoint(context.Background(), Point{
		ID: "PT-1", Name: "p1", Type: TypePressure, Unit: "MPa", Enabled: true,
		HighLimit: 10, LowLimit: 0, PhysicalMin: -5, PhysicalMax: 50,
	})
	return svc
}

func push001(svc *Service, n int) {
	now := time.Now()
	for i := 0; i < n; i++ {
		svc.Ingest(context.Background(), []Reading{{
			PointID: "PT-1", Value: 1.0 + float64(i%7), Ts: now.Add(time.Duration(i) * time.Second),
		}})
	}
}

// TestScadaSnapshotDetachedR001A: a store-level history snapshot must expose
// exactly the stored readings and stay stable when more arrive.
func TestScadaSnapshotDetachedR001A(t *testing.T) {
	svc := newRace001Svc(t, 10)
	push001(svc, 4)
	snap := svc.Store().History("PT-1")
	if len(snap) != 4 {
		t.Fatalf("want 4 readings, got %d", len(snap))
	}
	push001(svc, 7)
	if len(snap) != 4 {
		t.Fatalf("snapshot length mutated to %d after later writes", len(snap))
	}
	for i, rd := range snap {
		want := 1.0 + float64(i%7)
		if rd.Value != want {
			t.Fatalf("snapshot[%d] mutated: value=%g want %g", i, rd.Value, want)
		}
	}
}

// TestScadaServiceHistoryDetachedR001B: the service-level history must not
// alias the internal ring buffer either.
func TestScadaServiceHistoryDetachedR001B(t *testing.T) {
	svc := newRace001Svc(t, 10)
	push001(svc, 4)
	hist, err := svc.History(context.Background(), "PT-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 4 {
		t.Fatalf("want 4 readings, got %d", len(hist))
	}
	push001(svc, 7)
	if len(hist) != 4 {
		t.Fatalf("service history length mutated to %d after later writes", len(hist))
	}
	for i, rd := range hist {
		want := 1.0 + float64(i%7)
		if rd.Value != want {
			t.Fatalf("service history[%d] mutated: value=%g want %g", i, rd.Value, want)
		}
	}
}

// TestScadaConcurrentHistoryStableR001C: concurrent history reads while a
// writer ingests must not race or corrupt the returned snapshots.
func TestScadaConcurrentHistoryStableR001C(t *testing.T) {
	svc := newRace001Svc(t, 200)
	push001(svc, 10)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 1000; j++ {
				h, err := svc.History(context.Background(), "PT-1", 200)
				if err != nil {
					t.Errorf("history error: %v", err)
					return
				}
				if len(h) > 200 {
					t.Errorf("history longer than limit: %d", len(h))
					return
				}
				// force reading every element of the returned window
				sum := 0.0
				for _, rd := range h {
					sum += rd.Value
				}
				_ = sum
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		now := time.Now()
		for j := 0; j < 2000; j++ {
			svc.Ingest(context.Background(), []Reading{{
				PointID: "PT-1", Value: 1.0 + float64(j%7),
				Ts: now.Add(time.Duration(j) * time.Millisecond),
			}})
		}
	}()
	close(start)
	wg.Wait()
}

// TestScadaRulesDetachedR001E: the rule list returned by Rules must be a
// copy; mutating it must not corrupt the registered rules.
func TestScadaRulesDetachedR001E(t *testing.T) {
	svc := newRace001Svc(t, 10)
	if _, err := svc.AddRule(context.Background(), Rule{ID: "R1", PointID: "PT-1", Kind: RuleThreshold, Level: AlarmHigh, High: 8, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddRule(context.Background(), Rule{ID: "R2", PointID: "PT-1", Kind: RuleThreshold, Level: AlarmHigh, High: 9, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	rules := svc.Rules(context.Background(), "PT-1")
	if len(rules) != 2 {
		t.Fatalf("want 2 rules, got %d", len(rules))
	}
	// caller-side edit must not reach the store
	rules[0].Enabled = false
	again := svc.Rules(context.Background(), "PT-1")
	if len(again) != 2 {
		t.Fatalf("rule list length changed after caller edit: %d", len(again))
	}
	if !again[0].Enabled {
		t.Fatalf("caller edit corrupted the stored rule: rules[0].Enabled=false")
	}
}
