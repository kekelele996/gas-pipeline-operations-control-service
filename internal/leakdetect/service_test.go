package leakdetect

import (
	"context"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
	"gas-pipeline-operations-control-service/internal/scada"
)

func newSvc() (*Service, *platform.FakeClock) {
	c := platform.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	return NewService(c, Thresholds{DropRate: 0.15, Imbalance: 0.05, WindowMinutes: 30}), c
}

func readings(values []float64, t0 time.Time) []scada.Reading {
	out := make([]scada.Reading, len(values))
	for i, v := range values {
		out[i] = scada.Reading{PointID: "P", Value: v, Ts: t0.Add(time.Duration(i) * time.Minute)}
	}
	return out
}

func TestNoFalseAlarmOnNormalFluctuation(t *testing.T) {
	s, c := newSvc()
	up := readings([]float64{5.0, 5.01, 4.99, 5.0, 5.02}, c.Now())
	alert, err := s.AnalyzeSegment(context.Background(), "SEG1", up, up)
	if err != nil {
		t.Fatal(err)
	}
	if alert != nil {
		t.Fatalf("false alarm on normal fluctuation: %+v", alert)
	}
}

func TestSustainedDropTriggers(t *testing.T) {
	s, c := newSvc()
	// pressure falls steadily: 8.0 -> 6.0 over 10 minutes = 0.2 MPa/min > 0.15
	up := readings([]float64{8.0, 7.8, 7.6, 7.4, 7.2, 7.0, 6.8, 6.6, 6.4, 6.2}, c.Now())
	alert, err := s.AnalyzeSegment(context.Background(), "SEG1", up, up)
	if err != nil {
		t.Fatal(err)
	}
	if alert == nil {
		t.Fatalf("expected a leak alert")
	}
	if alert.Severity != SeveritySuspected && alert.Severity != SeverityProbable {
		t.Fatalf("severity = %s", alert.Severity)
	}
}

func TestImbalanceOnlyTriggersSuspected(t *testing.T) {
	s, c := newSvc()
	// flat upstream (no drop) but big downstream deviation -> imbalance only
	up := readings([]float64{5.0, 5.0, 5.0, 5.0}, c.Now())
	down := readings([]float64{4.0, 4.0, 4.0, 4.0}, c.Now()) // 20% imbalance > 5%
	alert, err := s.AnalyzeSegment(context.Background(), "SEG1", up, down)
	if err != nil {
		t.Fatal(err)
	}
	if alert == nil {
		t.Fatalf("expected imbalance alert")
	}
	if alert.Severity != SeveritySuspected {
		t.Fatalf("severity = %s, want suspected (only one indicator)", alert.Severity)
	}
}

func TestSinkReceivesAlert(t *testing.T) {
	s, c := newSvc()
	rec := &sinkRecorder{}
	s.AddSink(rec)
	up := readings([]float64{8.0, 7.0, 6.0, 5.0, 4.0}, c.Now())
	alert, _ := s.AnalyzeSegment(context.Background(), "SEG1", up, up)
	if alert == nil {
		t.Fatalf("expected alert")
	}
	if len(rec.alerts) != 1 {
		t.Fatalf("sink received %d alerts", len(rec.alerts))
	}
}

type sinkRecorder struct{ alerts []LeakAlert }

func (r *sinkRecorder) OnLeak(ctx context.Context, a LeakAlert) error {
	r.alerts = append(r.alerts, a)
	return nil
}
