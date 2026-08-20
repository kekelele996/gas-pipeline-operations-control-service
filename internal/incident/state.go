package incident

// Incident state machine: pending -> handling -> closed.

import "gas-pipeline-operations-control-service/internal/platform"

// Transitions encodes allowed incident-state moves.
var Transitions = map[string][]string{
	StatePending.String():  {StateHandling.String()},
	StateHandling.String(): {StateClosed.String()},
	StateClosed.String():   {},
}

// String returns the incident state name.
func (s State) String() string { return string(s) }

// AllStates lists every incident state.
func AllStates() []State {
	return []State{StatePending, StateHandling, StateClosed}
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
		return src, platform.Statef("incident: cannot transition %q -> %q", src, dst)
	}
	return dst, nil
}

// AllSeverities lists every severity level.
func AllSeverities() []Severity {
	return []Severity{SeverityInfo, SeverityWarning, SeveritySerious, SeverityCritical}
}
