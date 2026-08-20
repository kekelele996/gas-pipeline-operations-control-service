package httpapi

// permit.go: HTTP handlers for maintenance work permits.

import (
	"net/http"

	"gas-pipeline-operations-control-service/internal/permit"
)

func permitListHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.Permit.List(r.Context()))
	}
}

func permitApplyHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in permit.Input
		if err := decodeBody(r, &in); err != nil {
			mapError(w, err)
			return
		}
		p, err := deps.Permit.Apply(r.Context(), in)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, p)
	}
}

func permitGetHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := deps.Permit.Get(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

type permitActorReq struct {
	Approver string `json:"approver"`
}

func permitApproveHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body permitActorReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		p, err := deps.Permit.Approve(r.Context(), pathValue(r, "id"), body.Approver)
		if err != nil {
			// A rejected approval (overlapping permit, pending dispatch
			// order, open incident, or illegal state) is a real error:
			// surface it so the client sees the 409/conflict rather than a
			// silent success that leaves the permit in its prior state.
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func permitStartHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := deps.Permit.Start(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func permitCompleteHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := deps.Permit.Complete(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

type permitCancelReq struct {
	Reason string `json:"reason"`
}

func permitCancelHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body permitCancelReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		p, err := deps.Permit.Cancel(r.Context(), pathValue(r, "id"), body.Reason)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}
}

func permitExpireScanHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expired := deps.Permit.ExpireScan(r.Context())
		writeJSON(w, http.StatusOK, map[string]any{"expired": expired, "count": len(expired)})
	}
}
