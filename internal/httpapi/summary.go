package httpapi

// summary.go aggregates cross-domain counts for the operations dashboard.

import (
	"net/http"
	"time"

	"gas-pipeline-operations-control-service/internal/platform"
)

// summaryResponse is the body returned by GET /api/summary.
type summaryResponse struct {
	GeneratedAt          time.Time       `json:"generated_at"`
	Site                 string          `json:"site"`
	Network              networkSummary  `json:"network"`
	SCADA                scadaSummary    `json:"scada"`
	Metering             meteringSummary `json:"metering"`
	Incidents            int             `json:"open_incidents"`
	ActivePermits        int             `json:"active_permits"`
	ActiveOrders         int             `json:"active_orders"`
	PendingNotifications int             `json:"pending_notifications"`
}

type networkSummary struct {
	Segments    int `json:"segments"`
	Stations    int `json:"stations"`
	Compressors int `json:"compressors"`
	Valves      int `json:"valves"`
	Points      int `json:"points"`
}

type scadaSummary struct {
	Points   int `json:"points"`
	Readings int `json:"readings"`
	Alarms   int `json:"alarms"`
	Active   int `json:"active_alarms"`
}

type meteringSummary struct {
	Meters      int     `json:"meters"`
	TodayTotal  float64 `json:"today_total_std"`
	Settlements int     `json:"settlements"`
}

// summaryHandler returns the aggregated dashboard data.
func summaryHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		nSeg, nStn, nCmp, nVal, nPnt := deps.Network.Counts()
		pnt, rds, alm, act := deps.SCADA.Stats(ctx)
		today := time.Now().Format("2006-01-02")
		var todayTotal float64
		for _, dt := range deps.Metering.DailyForDate(ctx, today) {
			todayTotal += dt.StdVolume
		}
		nSettlements := len(deps.Metering.ListSettlements(ctx))
		// active permits: count non-terminal from list
		activePermits := 0
		for _, p := range deps.Permit.List(ctx) {
			if p.IsOpen() {
				activePermits++
			}
		}
		// active orders
		activeOrders := 0
		for _, o := range deps.Dispatch.List(ctx) {
			if o.State == "pending" || o.State == "issued" {
				activeOrders++
			}
		}
		// pending notifications: queued or retrying
		pendingNotifs := 0
		// Notify has no direct count method; approximate by reading the list
		// via a dedicated service call if available. We use Enqueue stats below.
		pendingNotifs = deps.Notify.CountPending(ctx)
		resp := summaryResponse{
			GeneratedAt:          time.Now(),
			Site:                 "GPL-CC-01",
			Network:              networkSummary{Segments: nSeg, Stations: nStn, Compressors: nCmp, Valves: nVal, Points: nPnt},
			SCADA:                scadaSummary{Points: pnt, Readings: rds, Alarms: alm, Active: act},
			Metering:             meteringSummary{Meters: len(deps.Metering.ListMeters(ctx)), TodayTotal: todayTotal, Settlements: nSettlements},
			Incidents:            deps.Incident.OpenCount(ctx),
			ActivePermits:        activePermits,
			ActiveOrders:         activeOrders,
			PendingNotifications: pendingNotifs,
		}
		platform.WriteJSONPretty(w, http.StatusOK, resp)
	}
}
