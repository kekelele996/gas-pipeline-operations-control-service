package notify

// Notification / alert push: recipients, channels, delivery state, retries.
// PushBatch simulates delivery against a configurable failure rate; the
// service exposes retry with exponential backoff.

import "time"

// Channel is the delivery mechanism for a notification.
type Channel string

const (
	ChannelSMS     Channel = "sms"
	ChannelEmail   Channel = "email"
	ChannelWebhook Channel = "webhook"
	ChannelPush    Channel = "push"
)

// State is the delivery state of a notification.
type State string

const (
	StateQueued   State = "queued"
	StateSent     State = "sent"
	StateFailed   State = "failed"
	StateRetrying State = "retrying"
)

// Notification is a queued outbound message.
type Notification struct {
	ID          string    `json:"id"`
	Recipient   string    `json:"recipient"`
	Channel     Channel   `json:"channel"`
	Subject     string    `json:"subject"`
	Body        string    `json:"body"`
	State       State     `json:"state"`
	Attempt     int       `json:"attempt"`
	MaxAttempts int       `json:"max_attempts"`
	NextRetryAt time.Time `json:"next_retry_at,omitempty"`
	SentAt      time.Time `json:"sent_at,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
