package permit

// Permit state machine: draft -> pending -> approved -> in_progress ->
// completed, with cancellation from any pre-execution state and expiry from
// approved (window passed without start).

import "gas-pipeline-operations-control-service/internal/platform"

// Transitions encodes allowed permit-state moves.
var Transitions = map[string][]string{
	StateDraft.String():      {StatePending.String(), StateCancelled.String()},
	StatePending.String():    {StateApproved.String(), StateCancelled.String(), StateExpired.String()},
	StateApproved.String():   {StateInProgress.String(), StateCancelled.String(), StateExpired.String()},
	StateInProgress.String(): {StateCompleted.String(), StateCancelled.String()},
	StateCompleted.String():  {},
	StateCancelled.String():  {},
	StateExpired.String():    {},
}

// String returns the permit state name.
func (s State) String() string { return string(s) }

// AllStates lists every permit state.
func AllStates() []State {
	return []State{StateDraft, StatePending, StateApproved, StateInProgress, StateCompleted, StateCancelled, StateExpired}
}

// CanTransition reports whether src->dst is a legal permit move.
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
		return src, platform.Statef("permit: cannot transition %q -> %q", src, dst)
	}
	return dst, nil
}
