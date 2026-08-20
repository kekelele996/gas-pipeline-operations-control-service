package network

// This file defines the topology data model for the pipeline network:
// Segments, Stations, Compressors, Valves, and measurement Points.
// All types are plain value types; state-machine constants live in state.go.

import "time"

// Segment represents a contiguous length of pipeline between two endpoints.
type Segment struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	From           string    `json:"from"` // upstream station id
	To             string    `json:"to"`   // downstream station id
	LengthKm       float64   `json:"length_km"`
	DiameterMm     float64   `json:"diameter_mm"`
	MAOPMPa        float64   `json:"maop_mpa"` // maximum allowable operating pressure
	Tier           string    `json:"tier"`     // "main" | "branch" | "lateral"
	Material       string    `json:"material"`
	CommissionedAt time.Time `json:"commissioned_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// StationType enumerates station classifications.
type StationType string

const (
	StationSource       StationType = "source"       // gas entry point
	StationCompressor   StationType = "compressor"   // compressor station
	StationMeter        StationType = "meter"        // custody-transfer meter station
	StationDelivery     StationType = "delivery"     // delivery / offtake
	StationIntersection StationType = "intersection" // pipeline junction
	StationStorage      StationType = "storage"      // underground storage
)

// Station is a physical site along the pipeline.
type Station struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Code           string      `json:"code"`
	Type           StationType `json:"type"`
	SegmentIDs     []string    `json:"segment_ids"` // segments connected to this station
	Latitude       float64     `json:"latitude"`
	Longitude      float64     `json:"longitude"`
	Region         string      `json:"region"`
	Operator       string      `json:"operator"`
	CommissionedAt time.Time   `json:"commissioned_at"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// CompressorState is the lifecycle state of a compressor unit.
type CompressorState string

const (
	CompressorRunning     CompressorState = "running"
	CompressorStopped     CompressorState = "stopped"
	CompressorMaintenance CompressorState = "maintenance"
	CompressorFaulted     CompressorState = "faulted"
)

// Compressor is a gas-compression unit at a compressor station.
type Compressor struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	StationID     string          `json:"station_id"`
	SegmentID     string          `json:"segment_id"`
	Model         string          `json:"model"`
	PowerMW       float64         `json:"power_mw"`
	State         CompressorState `json:"state"`
	LastStartedAt time.Time       `json:"last_started_at"`
	LastStoppedAt time.Time       `json:"last_stopped_at"`
	Reason        string          `json:"reason,omitempty"`
	UpdatedAt     time.Time       `json:"updated_at"`
	CreatedAt     time.Time       `json:"created_at"`
}

// ValveState is the position / health of a valve.
type ValveState string

const (
	ValveOpen       ValveState = "open"
	ValveClosed     ValveState = "closed"
	ValveThrottling ValveState = "throttling"
	ValveFault      ValveState = "fault"
)

// Valve controls or isolates flow on a segment or at a station.
type Valve struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	SegmentID          string     `json:"segment_id"`
	StationID          string     `json:"station_id"`
	Type               string     `json:"type"` // "block" | "control" | "relief" | "check"
	State              ValveState `json:"state"`
	Normal             string     `json:"normal"` // "open" | "closed" — fail-safe position
	RemoteControllable bool       `json:"remote_controllable"`
	UpdatedAt          time.Time  `json:"updated_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

// PointType enumerates the kinds of measurement points.
type PointType string

const (
	PointPressure     PointType = "pressure"
	PointFlow         PointType = "flow"
	PointTemperature  PointType = "temperature"
	PointDensity      PointType = "density"
	PointDifferential PointType = "differential"
)

// Point is a measurement point attached to a segment, station, or device.
type Point struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      PointType `json:"type"`
	Unit      string    `json:"unit"`
	SegmentID string    `json:"segment_id"`
	StationID string    `json:"station_id"`
	DeviceID  string    `json:"device_id,omitempty"` // compressor or valve id
	HighLimit float64   `json:"high_limit"`
	LowLimit  float64   `json:"low_limit"`
	RateLimit float64   `json:"rate_limit"` // max |Δ|/s
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Link describes how a point connects into the topology. It is derived data,
// not persisted; the store computes it on demand for downstream consumers
// such as the SCADA and leak-detection packages.
type Link struct {
	SegmentID string `json:"segment_id"`
	StationID string `json:"station_id"`
	DeviceID  string `json:"device_id,omitempty"`
}
