package contract

// Contract state machine: active <-> suspended, then expired/terminated.

import "gas-pipeline-operations-control-service/internal/platform"

// Transitions encodes allowed contract-state moves.
var Transitions = map[string][]string{
	StateActive.String():     {StateSuspended.String(), StateExpired.String(), StateTerminated.String()},
	StateSuspended.String():  {StateActive.String(), StateTerminated.String(), StateExpired.String()},
	StateExpired.String():    {},
	StateTerminated.String(): {},
}

// String returns the contract state name.
func (s State) String() string { return string(s) }

// AllStates lists every contract state.
func AllStates() []State {
	return []State{StateActive, StateSuspended, StateExpired, StateTerminated}
}

// CanTransition reports whether src->dst is a legal contract move.
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
		return src, platform.Statef("contract: cannot transition %q -> %q", src, dst)
	}
	return dst, nil
}
