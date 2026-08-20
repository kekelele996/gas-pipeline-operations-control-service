package httpapi

// network.go: HTTP handlers for pipeline topology (segments, stations,
// compressors, valves, points).

import (
	"net/http"

	"gas-pipeline-operations-control-service/internal/network"
)

func listSegmentsHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.Network.ListSegments(r.Context()))
	}
}

func upsertSegmentHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in network.SegmentInput
		if err := decodeBody(r, &in); err != nil {
			mapError(w, err)
			return
		}
		seg, err := deps.Network.UpsertSegment(r.Context(), in)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, seg)
	}
}

func getSegmentHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seg, err := deps.Network.GetSegment(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, seg)
	}
}

func listSegmentDevicesHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		devs, err := deps.Network.ListSegmentDevices(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, devs)
	}
}

func listStationsHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.Network.ListStations(r.Context()))
	}
}

func upsertStationHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in network.StationInput
		if err := decodeBody(r, &in); err != nil {
			mapError(w, err)
			return
		}
		st, err := deps.Network.UpsertStation(r.Context(), in)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, st)
	}
}

func getStationHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := deps.Network.GetStation(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, st)
	}
}

func stationSegmentsHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// network service exposes Store().StationSegments; use GetStation then
		// delegate to the store via the service. We expose segments via the
		// segment list filtered by station is overkill; instead return the
		// connected segment ids through the service Store().
		_ = pathValue(r, "id")
		// The NetworkService interface doesn't carry StationSegments; compute
		// from segments whose From/To equals the station id.
		stationID := pathValue(r, "id")
		var ids []string
		for _, s := range deps.Network.ListSegments(r.Context()) {
			if s.From == stationID || s.To == stationID {
				ids = append(ids, s.ID)
			}
		}
		writeJSON(w, http.StatusOK, ids)
	}
}

func listCompressorsHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.Network.ListCompressors(r.Context()))
	}
}

func upsertCompressorHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in network.CompressorInput
		if err := decodeBody(r, &in); err != nil {
			mapError(w, err)
			return
		}
		c, err := deps.Network.UpsertCompressor(r.Context(), in)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, c)
	}
}

func getCompressorHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := deps.Network.GetCompressor(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, c)
	}
}

type deviceStateReq struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

func changeCompressorStateHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body deviceStateReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		c, err := deps.Network.ChangeCompressorState(r.Context(), pathValue(r, "id"), network.CompressorState(body.State), body.Reason)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, c)
	}
}

func listValvesHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.Network.ListValves(r.Context()))
	}
}

func upsertValveHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in network.ValveInput
		if err := decodeBody(r, &in); err != nil {
			mapError(w, err)
			return
		}
		v, err := deps.Network.UpsertValve(r.Context(), in)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}
}

func getValveHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := deps.Network.GetValve(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}
}

func changeValveStateHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body deviceStateReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		v, err := deps.Network.ChangeValveState(r.Context(), pathValue(r, "id"), network.ValveState(body.State))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	}
}

func listPointsHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.Network.ListPoints(r.Context()))
	}
}

func upsertPointHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in network.PointInput
		if err := decodeBody(r, &in); err != nil {
			mapError(w, err)
			return
		}
		p, err := deps.Network.UpsertPoint(r.Context(), in)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func getPointHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := deps.Network.GetPoint(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

type togglePointReq struct {
	Enabled bool `json:"enabled"`
}

func togglePointHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body togglePointReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		p, err := deps.Network.TogglePoint(r.Context(), pathValue(r, "id"), body.Enabled)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}
