package nomination

// Shipper nomination (Nomination): a daily volume request against a contract.
// The nomination passes through a state machine and consumes contract capacity
// when submitted; execution debits the daily used volume; cancellation releases
// it.

import "time"

// State is the nomination lifecycle state.
type State string

const (
	StateDraft     State = "draft"
	StateSubmitted State = "submitted"
	StateHeld      State = "held"
	StateConfirmed State = "confirmed"
	StateExecuted  State = "executed"
	StateCancelled State = "cancelled"
)

// Nomination is a daily volume nomination against a contract.
type Nomination struct {
	ID          string    `json:"id"`
	ContractID  string    `json:"contract_id"`
	ShipperID   string    `json:"shipper_id"`
	Date        string    `json:"date"`   // the gas day, "2006-01-02"
	Volume      float64   `json:"volume"` // nominated volume, m³
	Path        []string  `json:"path"`   // segment ids the nomination traverses
	State       State     `json:"state"`
	Reserved    bool      `json:"reserved"` // whether contract capacity is currently held
	SubmittedBy string    `json:"submitted_by,omitempty"`
	ConfirmedBy string    `json:"confirmed_by,omitempty"`
	Note        string    `json:"note,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
