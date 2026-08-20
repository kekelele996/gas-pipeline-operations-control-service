package scada

// Alarm state machine for the SCADA package. Alarms progress
// active -> acknowledged -> resolved. Re-arming returns a new alarm.

import "gas-pipeline-operations-control-service/internal/platform"

// AlarmTransitions encodes allowed alarm-state moves.
var AlarmTransitions = map[string][]string{
	AlarmActive.String():   {AlarmAcked.String(), AlarmResolved.String()},
	AlarmAcked.String():    {AlarmResolved.String()},
	AlarmResolved.String(): {},
}

// String returns the alarm state name.
func (s AlarmState) String() string { return string(s) }

// AllAlarmStates lists every alarm state for validation / display.
func AllAlarmStates() []AlarmState {
	return []AlarmState{AlarmActive, AlarmAcked, AlarmResolved}
}

// ValidAlarmState reports whether s is a known alarm state.
func ValidAlarmState(s string) bool {
	for _, v := range AllAlarmStates() {
		if v.String() == s {
			return true
		}
	}
	return false
}

// CanTransition reports whether src->dst is a legal alarm-state move.
func CanTransition(src, dst AlarmState) bool {
	allowed, ok := AlarmTransitions[src.String()]
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
func MustTransition(src, dst AlarmState) (AlarmState, error) {
	if !CanTransition(src, dst) {
		return src, platform.Statef("alarm: cannot transition %q -> %q", src, dst)
	}
	return dst, nil
}
