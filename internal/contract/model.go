package contract

// Shipper contract: daily capacity contracts between the pipeline operator
// and shippers. Each contract tracks contracted daily volume, used capacity,
// and validity period. The nomination package consumes capacity here.

import "time"

// State is the contract lifecycle state.
type State string

const (
	StateActive     State = "active"
	StateSuspended  State = "suspended"
	StateExpired    State = "expired"
	StateTerminated State = "terminated"
)

// Contract is a capacity agreement with a shipper.
type Contract struct {
	ID             string    `json:"id"`
	ShipperID      string    `json:"shipper_id"`
	ShipperName    string    `json:"shipper_name"`
	Code           string    `json:"code"`
	DailyVolume    float64   `json:"daily_volume"` // contracted daily capacity, m³
	UsedVolume     float64   `json:"used_volume"`  // currently reserved, m³
	ValidFrom      time.Time `json:"valid_from"`
	ValidTo        time.Time `json:"valid_to"`
	State          State     `json:"state"`
	PathSegmentIDs []string  `json:"path_segment_ids"` // reserved-path segments
	Notes          string    `json:"notes,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Remaining returns the remaining reservable capacity for this contract.
func (c Contract) Remaining() float64 {
	r := c.DailyVolume - c.UsedVolume
	if r < 0 {
		return 0
	}
	return r
}

// IsActiveNow reports whether the contract is active at time t (in effect and
// not suspended/expired).
func (c Contract) IsActiveNow(t time.Time) bool {
	if c.State != StateActive {
		return false
	}
	if !c.ValidFrom.IsZero() && t.Before(c.ValidFrom) {
		return false
	}
	if !c.ValidTo.IsZero() && t.After(c.ValidTo) {
		return false
	}
	return true
}
