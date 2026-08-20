package httpapi

// incident.go: HTTP handlers for event/accident management.

import (
	"net/http"

	"gas-pipeline-operations-control-service/internal/incident"
)

func incidentListHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.Incident.List(r.Context()))
	}
}

func incidentReportHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in incident.Input
		if err := decodeBody(r, &in); err != nil {
			mapError(w, err)
			return
		}
		i, err := deps.Incident.Report(r.Context(), in)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, i)
	}
}

func incidentGetHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		i, err := deps.Incident.Get(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, i)
	}
}

type incidentAssigneeReq struct {
	Assignee string `json:"assignee"`
}

func incidentConfirmHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body incidentAssigneeReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		i, err := deps.Incident.Confirm(r.Context(), pathValue(r, "id"), body.Assignee)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, i)
	}
}

type addActionReq struct {
	Description string `json:"description"`
	Owner       string `json:"owner"`
}

func incidentAddActionHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body addActionReq
		if err := decodeBody(r, &body); err != nil {
			mapError(w, err)
			return
		}
		i, err := deps.Incident.AddAction(r.Context(), pathValue(r, "id"), body.Description, body.Owner)
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, i)
	}
}

func incidentCompleteActionHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		i, err := deps.Incident.CompleteAction(r.Context(), pathValue(r, "id"), pathValue(r, "actionId"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, i)
	}
}

func incidentCloseHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		i, err := deps.Incident.Close(r.Context(), pathValue(r, "id"))
		if err != nil {
			mapError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, i)
	}
}
