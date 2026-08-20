package dispatch

// Dispatch order state machine: pending -> issued -> executed, with revoke
// from pending or issued.

import "gas-pipeline-operations-control-service/internal/platform"

// Transitions encodes allowed order-state moves.
var Transitions = map[string][]string{
	StatePending.String():  {StateIssued.String(), StateRevoked.String()},
	StateIssued.String():   {StateExecuted.String(), StateRevoked.String()},
	StateExecuted.String(): {},
	StateRevoked.String():  {},
}

// String returns the order state name.
func (s State) String() string { return string(s) }

// AllStates lists every dispatch state.
func AllStates() []State {
	return []State{StatePending, StateIssued, StateExecuted, StateRevoked}
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
		return src, platform.Statef("order: cannot transition %q -> %q", src, dst)
	}
	return dst, nil
}

// AllTypes lists every dispatch order type.
func AllTypes() []Type {
	return []Type{
		TypeOpenValve, TypeCloseValve, TypeStartCompressor,
		TypeStopCompressor, TypeRaiseFlow, TypeLowerFlow,
	}
}

// TargetTypeFor returns the device target type for an order type.
func TargetTypeFor(t Type) string {
	switch t {
	case TypeStartCompressor, TypeStopCompressor:
		return "compressor"
	case TypeOpenValve, TypeCloseValve:
		return "valve"
	default:
		return "flow"
	}
}
