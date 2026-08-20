package leakdetect

// Leak-detection service: takes a snapshot of pressure readings per segment
// and computes a drop rate (linear regression slope over the window) and an
// upstream/downstream balance deviation. Thresholds come from config.

import (
	"context"
	"fmt"
	"math"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
	"gas-pipeline-operations-control-service/internal/scada"
)

// Thresholds configures the detector.
type Thresholds struct {
	DropRate      float64 // MPa/min
	Imbalance     float64 // fractional
	WindowMinutes int
}

// AlarmSink is notified when a leak alert is produced.
type AlarmSink interface {
	OnLeak(ctx context.Context, a LeakAlert) error
}

// Service implements leak detection.
type Service struct {
	clock      platform.Clock
	thresholds Thresholds
	sinks      []AlarmSink
}

// NewService builds a leak-detection service.
func NewService(clock platform.Clock, t Thresholds) *Service {
	if clock == nil {
		clock = platform.SystemClock{}
	}
	if t.WindowMinutes <= 0 {
		t.WindowMinutes = 30
	}
	if t.DropRate <= 0 {
		t.DropRate = 0.15
	}
	if t.Imbalance <= 0 {
		t.Imbalance = 0.05
	}
	return &Service{clock: clock, thresholds: t}
}

// AddSink registers a leak sink.
func (s *Service) AddSink(sink AlarmSink) { s.sinks = append(s.sinks, sink) }

// AnalyzeSegment evaluates one segment's pressure readings and returns a
// LeakAlert when thresholds are breached, or nil otherwise. upstream and
// downstream are the ordered pressure readings for the segment endpoints over
// the analysis window.
func (s *Service) AnalyzeSegment(ctx context.Context, segmentID string, upstream, downstream []scada.Reading) (*LeakAlert, error) {
	if segmentID == "" {
		return nil, platform.Invalidf("segment_id required")
	}
	ev := Evidence{SegmentID: segmentID, DropRateMax: s.thresholds.DropRate, ImbalanceMax: s.thresholds.Imbalance}

	if len(upstream) > 0 {
		ev.WindowStart = upstream[0].Ts
		ev.WindowEnd = upstream[len(upstream)-1].Ts
		ev.UpstreamP = upstream[len(upstream)-1].Value
	}
	if len(downstream) > 0 {
		if ev.WindowStart.IsZero() || downstream[0].Ts.Before(ev.WindowStart) {
			ev.WindowStart = downstream[0].Ts
		}
		if downstream[len(downstream)-1].Ts.After(ev.WindowEnd) {
			ev.WindowEnd = downstream[len(downstream)-1].Ts
		}
		ev.DownstreamP = downstream[len(downstream)-1].Value
	}
	ev.SampleCount = len(upstream) + len(downstream)

	// drop rate from a linear-regression slope of the upstream series (MPa/min)
	dropRate := 0.0
	if len(upstream) >= 2 {
		dropRate = -regressionSlopeMPaPerMin(upstream)
	}
	ev.DropRate = dropRate

	// balance: relative deviation between latest upstream and downstream
	imbalance := 0.0
	if ev.UpstreamP > 0 && ev.DownstreamP > 0 {
		imbalance = math.Abs(ev.UpstreamP-ev.DownstreamP) / ev.UpstreamP
	}
	ev.Imbalance = imbalance

	dropHit := dropRate > s.thresholds.DropRate
	balanceHit := imbalance > s.thresholds.Imbalance
	if !dropHit && !balanceHit {
		return nil, nil
	}
	sev := SeveritySuspected
	if dropHit && balanceHit {
		sev = SeverityProbable
	}
	alert := &LeakAlert{
		ID: platform.NewLeakAlertID(), SegmentID: segmentID, Severity: sev,
		Evidence: ev, Ts: s.clock.Now(),
		Message: fmt.Sprintf("leak %s on %s: drop %.3f MPa/min (max %.3f), imbalance %.3f (max %.3f)",
			sev, segmentID, dropRate, s.thresholds.DropRate, imbalance, s.thresholds.Imbalance),
	}
	for _, sink := range s.sinks {
		_ = sink.OnLeak(ctx, *alert)
	}
	return alert, nil
}

// regressionSlopeMPaPerMin fits a least-squares line to the readings
// (x=minutes from first sample, y=value) and returns the slope. A falling
// pressure yields a negative slope.
func regressionSlopeMPaPerMin(rs []scada.Reading) float64 {
	n := len(rs)
	if n < 2 {
		return 0
	}
	t0 := rs[0].Ts
	var sx, sy, sxx, sxy float64
	for _, r := range rs {
		x := r.Ts.Sub(t0).Minutes()
		y := r.Value
		sx += x
		sy += y
		sxx += x * x
		sxy += x * y
	}
	denom := float64(n)*sxx - sx*sx
	if denom == 0 {
		return 0
	}
	return (float64(n)*sxy - sx*sy) / denom
}

// Thresholds returns the configured thresholds (read-only).
func (s *Service) Thresholds() Thresholds { return s.thresholds }

// keep time referenced
var _ = time.Now
