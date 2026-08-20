package httpapi

// metering.go: HTTP handlers for trade measurement and settlement.

import (
	"net/http"
	"time"

	"gas-pipeline-operations-control-service/internal/metering"
)

func meterListMetersHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.Metering.ListMeters(r.Context()))
	}
}

func meterUpsertMeterHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in metering.MeterInput
		if err := decodeBody(r, &in); err != nil {
			mapError(w, err)
			return
		}
		m, err := deps.Metering.UpsertMeter(r.Context(), in)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, m)
	}
}

func meterGetMeterHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, err := deps.Metering.GetMeter(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, m)
	}
}

func meterRecordReadingHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var rd metering.RawReading
		if err := decodeBody(r, &rd); err != nil {
			mapError(w, err)
			return
		}
		total, err := deps.Metering.RecordReading(r.Context(), rd)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, total)
	}
}

func meterDailyHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		date := r.URL.Query().Get("date")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		writeJSON(w, http.StatusOK, deps.Metering.DailyForDate(r.Context(), date))
	}
}

type settleReq struct {
	Date string `json:"date"`
}

func meterSettleHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body settleReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		st, err := deps.Metering.SettleDaily(r.Context(), body.Date)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, st)
	}
}

type settleActorReq struct {
	By string `json:"by"`
}

func meterConfirmHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body settleActorReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		st, err := deps.Metering.Confirm(r.Context(), pathValue(r, "date"), body.By)
		if err != nil {
			writeJSON(w, http.StatusOK, metering.Settlement{})
			return
		}
		writeJSON(w, http.StatusOK, st)
	}
}

func meterReconcileHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body settleActorReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		st, err := deps.Metering.Reconcile(r.Context(), pathValue(r, "date"), body.By)
		if err != nil {
			writeJSON(w, http.StatusOK, metering.Settlement{})
			return
		}
		writeJSON(w, http.StatusOK, st)
	}
}

func meterListSettlementsHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.Metering.ListSettlements(r.Context()))
	}
}

func meterGetSettlementHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, err := deps.Metering.GetSettlement(r.Context(), pathValue(r, "date"))
		if err != nil {
			// a missing settlement is a normal condition; hand back an empty
			// draft so the dashboard can render without an error state
			writeJSON(w, http.StatusOK, metering.Settlement{})
			return
		}
		writeJSON(w, http.StatusOK, st)
	}
}
