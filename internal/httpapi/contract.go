package httpapi

// contract.go + nomination.go: HTTP handlers for capacity contracts and
// shipper nominations.

import (
	"net/http"

	"gas-pipeline-operations-control-service/internal/contract"
	"gas-pipeline-operations-control-service/internal/nomination"
)

// ---- contracts ----

func contractListHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if shipper := r.URL.Query().Get("shipper_id"); shipper != "" {
			writeJSON(w, http.StatusOK, deps.Contract.ListByShipper(r.Context(), shipper))
			return
		}
		writeJSON(w, http.StatusOK, deps.Contract.List(r.Context()))
	}
}

func contractCreateHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in contract.ContractInput
		if err := decodeBody(r, &in); err != nil {
			mapError(w, err)
			return
		}
		c, err := deps.Contract.Create(r.Context(), in)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, c)
	}
}

func contractGetHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := deps.Contract.Get(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, c)
	}
}

func contractRemainingHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rem, err := deps.Contract.Remaining(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"contract_id": pathValue(r, "id"), "remaining": rem})
	}
}

type contractStateReq struct {
	State string `json:"state"`
}

func contractSetStateHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body contractStateReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		c, err := deps.Contract.SetState(r.Context(), pathValue(r, "id"), contract.State(body.State))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, c)
	}
}

// ---- nominations ----

func nominationListHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.Nomination.List(r.Context()))
	}
}

func nominationCreateHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in nomination.Input
		if err := decodeBody(r, &in); err != nil {
			mapError(w, err)
			return
		}
		n, err := deps.Nomination.Create(r.Context(), in)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, n)
	}
}

func nominationGetHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := deps.Nomination.Get(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, n)
	}
}

type nominationActorReq struct {
	By string `json:"by"`
}

func nominationSubmitHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body nominationActorReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		n, err := deps.Nomination.Submit(r.Context(), pathValue(r, "id"), body.By)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, n)
	}
}


func nominationConfirmHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body nominationActorReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		n, err := deps.Nomination.Confirm(r.Context(), pathValue(r, "id"), body.By)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, n)
	}
}

func nominationExecuteHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := deps.Nomination.Execute(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, n)
	}
}

func nominationCancelHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := deps.Nomination.Cancel(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, n)
	}
}
