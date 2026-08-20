package audit

// Audit records who did what to which entity and when. It is an append-only
// log; entries are never edited or deleted, only queried.

import "time"

// Entry is a single audit log record.
type Entry struct {
	ID         string    `json:"id"`
	Actor      string    `json:"actor"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Detail     string    `json:"detail"`
	Result     string    `json:"result,omitempty"` // "ok" | "error: ..."
	Ts         time.Time `json:"ts"`
}

// Query filters for searching audit entries.
type Query struct {
	Actor      string
	Action     string
	TargetType string
	TargetID   string
	Since      time.Time
	Until      time.Time
	Limit      int
}
