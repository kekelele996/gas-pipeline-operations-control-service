package leakdetect

// Leak detection: aggregates pressure telemetry per segment and computes a
// pressure drop rate and an upstream/downstream mass-balance deviation. When
// either exceeds a threshold a LeakAlert is produced.

import "time"

// Severity is the leak alert severity.
type Severity string

const (
	SeveritySuspected Severity = "suspected" // one indicator triggered
	SeverityProbable  Severity = "probable"  // both indicators triggered
)

// Evidence is the supporting analysis behind a leak judgement.
type Evidence struct {
	SegmentID    string    `json:"segment_id"`
	DropRate     float64   `json:"drop_rate"` // MPa/min
	DropRateMax  float64   `json:"drop_rate_threshold"`
	Imbalance    float64   `json:"imbalance"` // fractional deviation
	ImbalanceMax float64   `json:"imbalance_threshold"`
	UpstreamP    float64   `json:"upstream_pressure"`
	DownstreamP  float64   `json:"downstream_pressure"`
	SampleCount  int       `json:"sample_count"`
	WindowStart  time.Time `json:"window_start"`
	WindowEnd    time.Time `json:"window_end"`
}

// LeakAlert is the result of a leak analysis.
type LeakAlert struct {
	ID        string    `json:"id"`
	SegmentID string    `json:"segment_id"`
	Severity  Severity  `json:"severity"`
	Evidence  Evidence  `json:"evidence"`
	Message   string    `json:"message"`
	Ts        time.Time `json:"ts"`
}
