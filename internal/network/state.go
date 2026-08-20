package network

// State-machine definitions for network devices. Compressors have a richer
// lifecycle than valves; both are expressed as explicit transition tables so
// the HTTP layer can reject illegal moves with a stable error category.

// CompressorTransitions encodes the allowed compressor state moves.
var CompressorTransitions = map[string][]string{
	CompressorRunning.String(): {
		CompressorStopped.String(),
		CompressorMaintenance.String(),
		CompressorFaulted.String(),
	},
	CompressorStopped.String(): {
		CompressorRunning.String(),
		CompressorMaintenance.String(),
		CompressorFaulted.String(),
	},
	CompressorMaintenance.String(): {
		CompressorStopped.String(),
		CompressorFaulted.String(),
	},
	CompressorFaulted.String(): {
		CompressorMaintenance.String(),
		CompressorStopped.String(),
	},
}

// ValveTransitions encodes the allowed valve state moves.
var ValveTransitions = map[string][]string{
	ValveOpen.String(): {
		ValveClosed.String(),
		ValveThrottling.String(),
		ValveFault.String(),
	},
	ValveClosed.String(): {
		ValveOpen.String(),
		ValveFault.String(),
	},
	ValveThrottling.String(): {
		ValveOpen.String(),
		ValveClosed.String(),
		ValveFault.String(),
	},
	ValveFault.String(): {
		ValveOpen.String(),
		ValveClosed.String(),
	},
}

// String returns the state name; present so CompressorState and ValveState can
// be used uniformly with the transition-table helpers.
func (s CompressorState) String() string { return string(s) }
func (s ValveState) String() string      { return string(s) }

// AllCompressorStates lists every compressor state for validation / display.
func AllCompressorStates() []CompressorState {
	return []CompressorState{
		CompressorRunning,
		CompressorStopped,
		CompressorMaintenance,
		CompressorFaulted,
	}
}

// AllValveStates lists every valve state for validation / display.
func AllValveStates() []ValveState {
	return []ValveState{
		ValveOpen,
		ValveClosed,
		ValveThrottling,
		ValveFault,
	}
}

// AllStationTypes lists every station type for validation / display.
func AllStationTypes() []StationType {
	return []StationType{
		StationSource,
		StationCompressor,
		StationMeter,
		StationDelivery,
		StationIntersection,
		StationStorage,
	}
}

// AllPointTypes lists every measurement point type.
func AllPointTypes() []PointType {
	return []PointType{
		PointPressure,
		PointFlow,
		PointTemperature,
		PointDensity,
		PointDifferential,
	}
}

// ValidCompressorState reports whether s is a known compressor state.
func ValidCompressorState(s string) bool {
	for _, v := range AllCompressorStates() {
		if v.String() == s {
			return true
		}
	}
	return false
}

// ValidValveState reports whether s is a known valve state.
func ValidValveState(s string) bool {
	for _, v := range AllValveStates() {
		if v.String() == s {
			return true
		}
	}
	return false
}
