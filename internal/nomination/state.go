package nomination

// Nomination state machine: draft -> submitted -> confirmed -> executed, with
// cancellation from any pre-execution state.

import "gas-pipeline-operations-control-service/internal/platform"

// Transitions encodes allowed nomination-state moves.
var Transitions = map[string][]string{
	StateDraft.String():     {StateSubmitted.String(), StateCancelled.String()},
	StateSubmitted.String(): {StateConfirmed.String(), StateHeld.String(), StateCancelled.String()},
	StateHeld.String():      {StateConfirmed.String(), StateCancelled.String()},
	StateConfirmed.String(): {StateExecuted.String(), StateCancelled.String()},
	StateExecuted.String():  {},
	StateCancelled.String(): {},
}

// String returns the nomination state name.
func (s State) String() string { return string(s) }

// AllStates lists every nomination state.
func AllStates() []State {
	return []State{StateDraft, StateSubmitted, StateHeld, StateConfirmed, StateExecuted, StateCancelled}
}

// CanTransition reports whether src->dst is a legal nomination move.
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
		return src, platform.Statef("nomination: cannot transition %q -> %q", src, dst)
	}
	return dst, nil
}
