package permit

// Maintenance work permit: authorizes work on a segment within a time window.
// The permit passes through an approval + execution lifecycle and is checked
// against in-flight dispatch orders and incidents for the same segment.

import "time"

// State is the permit lifecycle state.
type State string

const (
	StateDraft      State = "draft"
	StatePending    State = "pending"
	StateApproved   State = "approved"
	StateInProgress State = "in_progress"
	StateCompleted  State = "completed"
	StateCancelled  State = "cancelled"
	StateExpired    State = "expired"
)

// Permit is a maintenance work permit.
type Permit struct {
	ID           string    `json:"id"`
	SegmentID    string    `json:"segment_id"`
	StationID    string    `json:"station_id,omitempty"`
	WorkType     string    `json:"work_type"` // "pigging" | "welding" | "inspection" | "repair" | "excavation"
	Title        string    `json:"title"`
	Applicant    string    `json:"applicant"`
	Approver     string    `json:"approver,omitempty"`
	WindowStart  time.Time `json:"window_start"`
	WindowEnd    time.Time `json:"window_end"`
	State        State     `json:"state"`
	Reason       string    `json:"reason,omitempty"`
	CancelReason string    `json:"cancel_reason,omitempty"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// IsOpen reports whether the permit is in an active (non-terminal) state.
func (p Permit) IsOpen() bool {
	switch p.State {
	case StateDraft, StatePending, StateApproved, StateInProgress:
		return true
	}
	return false
}
