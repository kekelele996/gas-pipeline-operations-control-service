package dispatch

// Operating dispatch order: a control instruction to a device (open/close
// valve, start/stop compressor, raise/lower flow). The order flows through
// pending -> issued -> executed, with revocation.

import "time"

// Type is the kind of dispatch order.
type Type string

const (
	TypeOpenValve       Type = "open_valve"
	TypeCloseValve      Type = "close_valve"
	TypeStartCompressor Type = "start_compressor"
	TypeStopCompressor  Type = "stop_compressor"
	TypeRaiseFlow       Type = "raise_flow"
	TypeLowerFlow       Type = "lower_flow"
)

// State is the dispatch order lifecycle state.
type State string

const (
	StatePending  State = "pending"
	StateIssued   State = "issued"
	StateExecuted State = "executed"
	StateRevoked  State = "revoked"
)

// Order is a single dispatch instruction.
type Order struct {
	ID         string    `json:"id"`
	Type       Type      `json:"type"`
	TargetType string    `json:"target_type"` // "compressor" | "valve"
	TargetID   string    `json:"target_id"`
	SegmentID  string    `json:"segment_id"`
	Value      float64   `json:"value,omitempty"` // for raise/lower flow
	Priority   int       `json:"priority"`
	Reason     string    `json:"reason"`
	IssuedBy   string    `json:"issued_by,omitempty"`
	IssuedAt   time.Time `json:"issued_at,omitempty"`
	ExecutedAt time.Time `json:"executed_at,omitempty"`
	State      State     `json:"state"`
	Outcome    string    `json:"outcome,omitempty"` // recorded on execution
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
