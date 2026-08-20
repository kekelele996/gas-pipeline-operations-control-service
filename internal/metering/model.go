package metering

// Trade measurement: meter points, raw volume readings, temperature-banded
// correction factors, daily totals, and settlement documents.

import "time"

// Meter is a custody-transfer measurement point. Each meter has a set of
// temperature-banded correction factors used to convert raw (line) volume to
// standard volume.
type Meter struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	SegmentID    string             `json:"segment_id"`
	StationID    string             `json:"station_id"`
	Unit         string             `json:"unit"`
	PressureBase float64            `json:"pressure_base"` // standard reference pressure, MPa
	TempBase     float64            `json:"temp_base"`     // standard reference temperature, °C
	Factors      []CorrectionFactor `json:"factors"`       // sorted by TempLo ascending
	Direction    string             `json:"direction"`     // "receipt" | "delivery"
	CustomerID   string             `json:"customer_id,omitempty"`
	Enabled      bool               `json:"enabled"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// CorrectionFactor converts raw volume to standard volume within a temperature
// band [TempLo, TempHi). Standard = raw * Factor.
type CorrectionFactor struct {
	TempLo float64 `json:"temp_lo"` // °C, inclusive lower bound
	TempHi float64 `json:"temp_hi"` // °C, exclusive upper bound
	Factor float64 `json:"factor"`  // multiplier
}

// RawReading is an uncorrected meter reading at a moment in time.
type RawReading struct {
	MeterID     string    `json:"meter_id"`
	Value       float64   `json:"value"`       // raw volume, m³
	Temperature float64   `json:"temperature"` // °C
	Pressure    float64   `json:"pressure"`    // MPa (line)
	Ts          time.Time `json:"ts"`
}

// DailyTotal is the running accumulation for a meter over a calendar day.
type DailyTotal struct {
	MeterID      string    `json:"meter_id"`
	Date         string    `json:"date"` // "2006-01-02"
	RawVolume    float64   `json:"raw_volume"`
	StdVolume    float64   `json:"std_volume"`
	ReadingCount int       `json:"reading_count"`
	FirstTs      time.Time `json:"first_ts"`
	LastTs       time.Time `json:"last_ts"`
	LastRaw      float64   `json:"last_raw"` // last raw value, for delta accounting
	LastStd      float64   `json:"last_std"` // last std value
}

// SettlementState is the lifecycle of a settlement document.
type SettlementState string

const (
	SettlementDraft      SettlementState = "draft"
	SettlementConfirmed  SettlementState = "confirmed"
	SettlementReconciled SettlementState = "reconciled"
)

// LineItem is one meter's totalled contribution to a settlement.
type LineItem struct {
	MeterID      string  `json:"meter_id"`
	Name         string  `json:"name"`
	RawVolume    float64 `json:"raw_volume"`
	StdVolume    float64 `json:"std_volume"`
	ReadingCount int     `json:"reading_count"`
}

// Settlement is the daily trade settlement document aggregating all meters.
type Settlement struct {
	ID           string          `json:"id"`
	Date         string          `json:"date"`
	State        SettlementState `json:"state"`
	Items        []LineItem      `json:"items"`
	TotalRaw     float64         `json:"total_raw"`
	TotalStd     float64         `json:"total_std"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	ConfirmedBy  string          `json:"confirmed_by,omitempty"`
	ReconciledBy string          `json:"reconciled_by,omitempty"`
	Note         string          `json:"note,omitempty"`
}
