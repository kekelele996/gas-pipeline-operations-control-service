package httpapi

// dispatch.go: HTTP handlers for operating dispatch orders.

import (
	"net/http"

	"gas-pipeline-operations-control-service/internal/dispatch"
)

func orderListHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.Dispatch.List(r.Context()))
	}
}

func orderCreateHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in dispatch.Input
		if err := decodeBody(r, &in); err != nil {
			mapError(w, err)
			return
		}
		o, err := deps.Dispatch.Create(r.Context(), in)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, o)
	}
}

func orderGetHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		o, err := deps.Dispatch.Get(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, o)
	}
}

type orderActorReq struct {
	By string `json:"by"`
}

func orderIssueHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body orderActorReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		o, err := deps.Dispatch.Issue(r.Context(), pathValue(r, "id"), body.By)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, o)
	}
}

func orderExecuteHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		o, err := deps.Dispatch.Execute(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, o)
	}
}

func orderRevokeHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		o, err := deps.Dispatch.Revoke(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, o)
	}
}
