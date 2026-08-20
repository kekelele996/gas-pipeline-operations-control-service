package httpapi

// router.go registers all HTTP routes onto a standard library ServeMux.
// The pattern layout follows the generation prompt: /health, /api/summary,
// and one handler file per domain under /api/...

import (
	"net/http"
	"os"
	"strings"

	"gas-pipeline-operations-control-service/internal/platform"
)

// NewRouter wires every route onto a ServeMux and returns it.
func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()

	// health + summary
	mux.HandleFunc("GET /health", healthHandler(deps))
	mux.HandleFunc("GET /api/summary", summaryHandler(deps))

	// network
	mux.HandleFunc("GET /api/segments", listSegmentsHandler(deps))
	mux.HandleFunc("POST /api/segments", upsertSegmentHandler(deps))
	mux.HandleFunc("GET /api/segments/{id}", getSegmentHandler(deps))
	mux.HandleFunc("GET /api/segments/{id}/devices", listSegmentDevicesHandler(deps))
	mux.HandleFunc("POST /api/segments/{id}/limits", recordSegmentLimitHandler(deps))
	mux.HandleFunc("GET /api/stations", listStationsHandler(deps))
	mux.HandleFunc("POST /api/stations", upsertStationHandler(deps))
	mux.HandleFunc("GET /api/stations/{id}", getStationHandler(deps))
	mux.HandleFunc("GET /api/stations/{id}/segments", stationSegmentsHandler(deps))
	mux.HandleFunc("GET /api/compressors", listCompressorsHandler(deps))
	mux.HandleFunc("POST /api/compressors", upsertCompressorHandler(deps))
	mux.HandleFunc("GET /api/compressors/{id}", getCompressorHandler(deps))
	mux.HandleFunc("POST /api/compressors/{id}/state", changeCompressorStateHandler(deps))
	mux.HandleFunc("GET /api/valves", listValvesHandler(deps))
	mux.HandleFunc("POST /api/valves", upsertValveHandler(deps))
	mux.HandleFunc("GET /api/valves/{id}", getValveHandler(deps))
	mux.HandleFunc("POST /api/valves/{id}/state", changeValveStateHandler(deps))
	mux.HandleFunc("GET /api/points", listPointsHandler(deps))
	mux.HandleFunc("POST /api/points", upsertPointHandler(deps))
	mux.HandleFunc("GET /api/points/{id}", getPointHandler(deps))
	mux.HandleFunc("POST /api/points/{id}/toggle", togglePointHandler(deps))

	// scada
	mux.HandleFunc("GET /api/scada/points", scadaListPointsHandler(deps))
	mux.HandleFunc("GET /api/scada/points/{id}", scadaGetPointHandler(deps))
	mux.HandleFunc("GET /api/scada/points/{id}/history", scadaHistoryHandler(deps))
	mux.HandleFunc("GET /api/scada/points/{id}/latest", scadaLatestHandler(deps))
	mux.HandleFunc("POST /api/telemetry", scadaIngestHandler(deps))
	mux.HandleFunc("GET /api/alarms", scadaAlarmsHandler(deps))
	mux.HandleFunc("POST /api/alarms/{id}/ack", scadaAckAlarmHandler(deps))
	mux.HandleFunc("POST /api/alarms/{id}/resolve", scadaResolveAlarmHandler(deps))

	// metering
	mux.HandleFunc("GET /api/metering/meters", meterListMetersHandler(deps))
	mux.HandleFunc("POST /api/metering/meters", meterUpsertMeterHandler(deps))
	mux.HandleFunc("GET /api/metering/meters/{id}", meterGetMeterHandler(deps))
	mux.HandleFunc("POST /api/metering/readings", meterRecordReadingHandler(deps))
	mux.HandleFunc("GET /api/metering/daily", meterDailyHandler(deps))
	mux.HandleFunc("POST /api/metering/settle", meterSettleHandler(deps))
	mux.HandleFunc("POST /api/metering/settle/{date}/confirm", meterConfirmHandler(deps))
	mux.HandleFunc("POST /api/metering/settle/{date}/reconcile", meterReconcileHandler(deps))
	mux.HandleFunc("GET /api/metering/settlements", meterListSettlementsHandler(deps))
	mux.HandleFunc("GET /api/metering/settlements/{date}", meterGetSettlementHandler(deps))

	// contracts
	mux.HandleFunc("GET /api/contracts", contractListHandler(deps))
	mux.HandleFunc("POST /api/contracts", contractCreateHandler(deps))
	mux.HandleFunc("GET /api/contracts/{id}", contractGetHandler(deps))
	mux.HandleFunc("GET /api/contracts/{id}/remaining", contractRemainingHandler(deps))
	mux.HandleFunc("POST /api/contracts/{id}/state", contractSetStateHandler(deps))

	// nominations
	mux.HandleFunc("GET /api/nominations", nominationListHandler(deps))
	mux.HandleFunc("POST /api/nominations", nominationCreateHandler(deps))
	mux.HandleFunc("GET /api/nominations/{id}", nominationGetHandler(deps))
	mux.HandleFunc("POST /api/nominations/{id}/submit", nominationSubmitHandler(deps))
	mux.HandleFunc("POST /api/nominations/{id}/confirm", nominationConfirmHandler(deps))
	mux.HandleFunc("POST /api/nominations/{id}/execute", nominationExecuteHandler(deps))
	mux.HandleFunc("POST /api/nominations/{id}/cancel", nominationCancelHandler(deps))

	// permits
	mux.HandleFunc("GET /api/permits", permitListHandler(deps))
	mux.HandleFunc("POST /api/permits", permitApplyHandler(deps))
	mux.HandleFunc("GET /api/permits/{id}", permitGetHandler(deps))
	mux.HandleFunc("POST /api/permits/{id}/approve", permitApproveHandler(deps))
	mux.HandleFunc("POST /api/permits/{id}/start", permitStartHandler(deps))
	mux.HandleFunc("POST /api/permits/{id}/complete", permitCompleteHandler(deps))
	mux.HandleFunc("POST /api/permits/{id}/cancel", permitCancelHandler(deps))
	mux.HandleFunc("POST /api/permits/expire-scan", permitExpireScanHandler(deps))

	// dispatch orders
	mux.HandleFunc("GET /api/orders", orderListHandler(deps))
	mux.HandleFunc("POST /api/orders", orderCreateHandler(deps))
	mux.HandleFunc("GET /api/orders/{id}", orderGetHandler(deps))
	mux.HandleFunc("POST /api/orders/{id}/issue", orderIssueHandler(deps))
	mux.HandleFunc("POST /api/orders/{id}/execute", orderExecuteHandler(deps))
	mux.HandleFunc("POST /api/orders/{id}/revoke", orderRevokeHandler(deps))

	// incidents
	mux.HandleFunc("GET /api/incidents", incidentListHandler(deps))
	mux.HandleFunc("POST /api/incidents", incidentReportHandler(deps))
	mux.HandleFunc("GET /api/incidents/{id}", incidentGetHandler(deps))
	mux.HandleFunc("POST /api/incidents/{id}/confirm", incidentConfirmHandler(deps))
	mux.HandleFunc("POST /api/incidents/{id}/actions", incidentAddActionHandler(deps))
	mux.HandleFunc("POST /api/incidents/{id}/actions/{actionId}/complete", incidentCompleteActionHandler(deps))
	mux.HandleFunc("POST /api/incidents/{id}/close", incidentCloseHandler(deps))

	// leak detect
	mux.HandleFunc("GET /api/leaks/analyze/{segmentId}", leakAnalyzeHandler(deps))

	// audit
	mux.HandleFunc("GET /api/audit", auditQueryHandler(deps))
	mux.HandleFunc("GET /api/audit/summary", auditSummaryHandler(deps))

	// notify
	mux.HandleFunc("GET /api/notifications", notifyListHandler(deps))
	mux.HandleFunc("POST /api/notifications", notifyEnqueueHandler(deps))
	mux.HandleFunc("POST /api/notifications/push", notifyPushHandler(deps))
	mux.HandleFunc("POST /api/notifications/retry", notifyRetryHandler(deps))

	// static web dashboard (served from the web/ directory on disk)
	mux.HandleFunc("GET /", webDashboardHandler)

	return mux
}

// healthHandler reports liveness. It returns 200 with a small JSON body.
func healthHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		platform.WriteJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "gas-pipeline-operations-control-service",
		})
	}
}

// webDashboardHandler serves the static web/ directory. The root path serves
// index.html; known assets (app.js, style.css) serve from disk; any other
// non-API path falls back to index.html for client-side routing.
func webDashboardHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "api/") {
		http.NotFound(w, r)
		return
	}
	p := r.URL.Path
	if p == "/" || p == "" {
		serveAsset(w, r, "web/index.html", "text/html; charset=utf-8")
		return
	}
	clean := strings.TrimPrefix(p, "/")
	switch clean {
	case "index.html":
		serveAsset(w, r, "web/index.html", "text/html; charset=utf-8")
	case "app.js":
		serveAsset(w, r, "web/app.js", "text/javascript; charset=utf-8")
	case "style.css":
		serveAsset(w, r, "web/style.css", "text/css; charset=utf-8")
	default:
		// SPA fallback: serve the dashboard for any unknown non-API path.
		serveAsset(w, r, "web/index.html", "text/html; charset=utf-8")
	}
}

// serveAsset reads a file from disk and writes it with the given content type.
// Using this instead of http.ServeFile avoids ServeFile's redirect behavior
// for index.html, which would 301 to "./index.html" on the root path.
func serveAsset(w http.ResponseWriter, r *http.Request, path, contentType string) {
	data, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}
