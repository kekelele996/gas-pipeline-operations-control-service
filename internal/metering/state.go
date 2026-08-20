package metering

// Settlement state machine: draft -> confirmed -> reconciled.

import "gas-pipeline-operations-control-service/internal/platform"

// SettlementTransitions encodes allowed settlement-state moves.
var SettlementTransitions = map[string][]string{
	SettlementDraft.String():      {SettlementConfirmed.String()},
	SettlementConfirmed.String():  {SettlementReconciled.String()},
	SettlementReconciled.String(): {},
}

// String returns the settlement state name.
func (s SettlementState) String() string { return string(s) }

// AllSettlementStates lists every settlement state.
func AllSettlementStates() []SettlementState {
	return []SettlementState{SettlementDraft, SettlementConfirmed, SettlementReconciled}
}

// CanTransition reports whether src->dst is a legal settlement move.
func CanTransition(src, dst SettlementState) bool {
	allowed, ok := SettlementTransitions[src.String()]
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
func MustTransition(src, dst SettlementState) (SettlementState, error) {
	if !CanTransition(src, dst) {
		return src, platform.Statef("settlement: cannot transition %q -> %q", src, dst)
	}
	return dst, nil
}
