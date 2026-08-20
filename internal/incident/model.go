package incident

// Incident / accident management: an incident is raised from an alarm or a
// manual report, moves through confirmation -> handling -> closed, and tracks
// a list of remediation actions that must all be complete before closure.

import "time"

// State is the incident lifecycle state.
type State string

const (
	StatePending  State = "pending"
	StateHandling State = "handling"
	StateClosed   State = "closed"
)

// Severity is the incident severity level.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeveritySerious  Severity = "serious"
	SeverityCritical Severity = "critical"
)

// ActionItem is a remediation step on an incident.
type ActionItem struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Owner       string    `json:"owner"`
	Done        bool      `json:"done"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// Incident is an event/accident record.
type Incident struct {
	ID          string       `json:"id"`
	SegmentID   string       `json:"segment_id,omitempty"`
	DeviceID    string       `json:"device_id,omitempty"`
	StationID   string       `json:"station_id,omitempty"`
	Severity    Severity     `json:"severity"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	AlarmIDs    []string     `json:"alarm_ids,omitempty"`
	Actions     []ActionItem `json:"actions"`
	State       State        `json:"state"`
	Reporter    string       `json:"reporter"`
	Assignee    string       `json:"assignee,omitempty"`
	ClosedAt    time.Time    `json:"closed_at,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// AllActionsComplete reports whether every remediation action is done.
func (i Incident) AllActionsComplete() bool {
	if len(i.Actions) == 0 {
		return false
	}
	for _, a := range i.Actions {
		if !a.Done {
			return false
		}
	}
	return true
}
