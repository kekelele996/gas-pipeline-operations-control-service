package httpapi

// scada.go: HTTP handlers for telemetry ingestion, point history, and alarms.

import (
	"net/http"

	"gas-pipeline-operations-control-service/internal/scada"
)

func scadaListPointsHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.SCADA.ListPoints(r.Context()))
	}
}

func scadaGetPointHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := deps.SCADA.Point(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func scadaHistoryHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := queryInt(r, "limit", 100)
		hist, err := deps.SCADA.History(r.Context(), pathValue(r, "id"), limit)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, hist)
	}
}

func scadaLatestHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rd, err := deps.SCADA.Latest(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, rd)
	}
}

type telemetryBatch struct {
	Readings []scada.Reading `json:"readings"`
}

func scadaIngestHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var batch telemetryBatch
		if err := decodeBody(r, &batch); err != nil {
			mapError(w, err)
			return
		}
		results := deps.SCADA.Ingest(r.Context(), batch.Readings)
		writeJSON(w, http.StatusOK, map[string]any{
			"ingested": len(results),
			"results":  results,
		})
	}
}

func scadaAlarmsHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pointID := r.URL.Query().Get("point_id")
		onlyActive := queryBool(r, "active")
		alarms := deps.SCADA.Alarms(r.Context(), pointID, onlyActive)
		writeJSON(w, http.StatusOK, alarms)
	}
}

type ackReq struct {
	By string `json:"by"`
}

func scadaAckAlarmHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body ackReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		a, err := deps.SCADA.AckAlarm(r.Context(), pathValue(r, "id"), body.By)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, a)
	}
}

func scadaResolveAlarmHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, err := deps.SCADA.ResolveAlarm(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, a)
	}
}
