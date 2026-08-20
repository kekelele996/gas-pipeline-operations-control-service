package scada

// SCADA model: measurement points, readings, alarms, and alarm rules.
// The package keeps readings in a per-point ring buffer and evaluates alarms
// on ingest using threshold and rate-of-change rules.

import "time"

// PointType re-exports the measurement-point kinds. We define them locally so
// SCADA can be reasoned about without importing network.
type PointType string

const (
	TypePressure     PointType = "pressure"
	TypeFlow         PointType = "flow"
	TypeTemperature  PointType = "temperature"
	TypeDensity      PointType = "density"
	TypeDifferential PointType = "differential"
)

// Point is the SCADA-side definition of a measurement point, mirroring the
// network.Point but kept independent so the SCADA store can ingest points
// from any source. Limits drive alarm evaluation.
type Point struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      PointType `json:"type"`
	Unit      string    `json:"unit"`
	SegmentID string    `json:"segment_id"`
	HighLimit float64   `json:"high_limit"`
	LowLimit  float64   `json:"low_limit"`
	RateLimit float64   `json:"rate_limit"`
	Enabled   bool      `json:"enabled"`
	// PhysicalMin/Max bound plausibility checks; readings outside these are
	// treated as bad values and dropped before storage.
	PhysicalMin float64   `json:"physical_min"`
	PhysicalMax float64   `json:"physical_max"`
	CreatedAt   time.Time `json:"created_at"`
}

// Reading is a single timestamped sample for a point.
type Reading struct {
	PointID string    `json:"point_id"`
	Value   float64   `json:"value"`
	Ts      time.Time `json:"ts"`
	// Quality is a coarse flag: "good" | "uncertain" | "bad".
	Quality string `json:"quality"`
}

// AlarmLevel orders alarm severity.
type AlarmLevel string

const (
	AlarmLow      AlarmLevel = "low"
	AlarmHigh     AlarmLevel = "high"
	AlarmCritical AlarmLevel = "critical"
)

// AlarmState is the lifecycle of an alarm.
type AlarmState string

const (
	AlarmActive   AlarmState = "active"
	AlarmAcked    AlarmState = "acknowledged"
	AlarmResolved AlarmState = "resolved"
)

// Alarm is produced by rule evaluation during ingest.
type Alarm struct {
	ID         string     `json:"id"`
	PointID    string     `json:"point_id"`
	Level      AlarmLevel `json:"level"`
	Message    string     `json:"message"`
	Ts         time.Time  `json:"ts"`
	State      AlarmState `json:"state"`
	RuleID     string     `json:"rule_id,omitempty"`
	Value      float64    `json:"value,omitempty"`
	AckedBy    string     `json:"acked_by,omitempty"`
	ResolvedAt time.Time  `json:"resolved_at,omitempty"`
}

// Rule is an alarm evaluation rule. The engine supports threshold and
// rate rules; a rule's Kind selects the evaluator.
type Rule struct {
	ID      string     `json:"id"`
	PointID string     `json:"point_id"`
	Kind    string     `json:"kind"` // "threshold" | "rate"
	Level   AlarmLevel `json:"level"`
	Message string     `json:"message"`
	Enabled bool       `json:"enabled"`
	// Threshold fields
	High float64 `json:"high,omitempty"`
	Low  float64 `json:"low,omitempty"`
	// Rate field: max absolute change per second
	Rate float64 `json:"rate,omitempty"`
}

// RuleKind constants.
const (
	RuleThreshold = "threshold"
	RuleRate      = "rate"
)
