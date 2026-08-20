package notify

// Notification state machine: queued -> sent (terminal), or queued ->
// retrying -> {sent, failed}. Failed is terminal once attempts are exhausted.

import "gas-pipeline-operations-control-service/internal/platform"

// Transitions encodes allowed notification-state moves.
var Transitions = map[string][]string{
	StateQueued.String():   {StateSent.String(), StateRetrying.String(), StateFailed.String()},
	StateRetrying.String(): {StateSent.String(), StateRetrying.String(), StateFailed.String()},
	StateSent.String():     {},
	StateFailed.String():   {},
}

// String returns the notification state name.
func (s State) String() string { return string(s) }

// AllStates lists every notification state.
func AllStates() []State {
	return []State{StateQueued, StateSent, StateRetrying, StateFailed}
}

// CanTransition reports whether src->dst is a legal move.
func CanTransition(src, dst State) bool {
	allowed, ok := Transitions[src.String()]
	if !ok {
		return false
	}
	for _, d := range allowed {
		if d == dst.String() {
			return true
		}
	}
	return false
}

// MustTransition validates src->dst and returns dst, or an ErrState.
func MustTransition(src, dst State) (State, error) {
	if !CanTransition(src, dst) {
		return src, platform.Statef("notification: cannot transition %q -> %q", src, dst)
	}
	return dst, nil
}

// AllChannels lists every delivery channel.
func AllChannels() []Channel {
	return []Channel{ChannelSMS, ChannelEmail, ChannelWebhook, ChannelPush}
}
