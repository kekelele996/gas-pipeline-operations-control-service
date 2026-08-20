package httpapi

// audit.go + notify.go: HTTP handlers for audit log query and notification ops.

import (
	"net/http"
	"strconv"
	"time"

	"gas-pipeline-operations-control-service/internal/audit"
	"gas-pipeline-operations-control-service/internal/notify"
)

// ---- audit ----

func auditQueryHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := audit.Query{
			Actor:      r.URL.Query().Get("actor"),
			Action:     r.URL.Query().Get("action"),
			TargetType: r.URL.Query().Get("target_type"),
			TargetID:   r.URL.Query().Get("target_id"),
			Limit:      queryInt(r, "limit", 100),
		}
		if s := r.URL.Query().Get("since"); s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				q.Since = t
			}
		}
		if s := r.URL.Query().Get("until"); s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				q.Until = t
			}
		}
		writeJSON(w, http.StatusOK, deps.Audit.Query(r.Context(), q))
	}
}

func auditSummaryHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.Audit.Summary(r.Context()))
	}
}

// ---- notify ----

func notifyListHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.Notify.List(r.Context()))
	}
}

type enqueueReq struct {
	Recipient string         `json:"recipient"`
	Channel   notify.Channel `json:"channel"`
	Subject   string         `json:"subject"`
	Body      string         `json:"body"`
}

func notifyEnqueueHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body enqueueReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		n, err := deps.Notify.Enqueue(r.Context(), body.Recipient, body.Channel, body.Subject, body.Body)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, n)
	}
}

func notifyPushHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sent, failed, err := deps.Notify.PushBatch(r.Context())
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sent": sent, "failed": failed})
	}
}

func notifyRetryHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n := deps.Notify.RetryFailed(r.Context())
		writeJSON(w, http.StatusOK, map[string]any{"retried": n})
	}
}

// keep strconv referenced for queryInt robustness
var _ = strconv.Atoi
