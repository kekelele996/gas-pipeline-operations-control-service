package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"gas-pipeline-operations-control-service/internal/incident"
)

// TestIncidentOpenFilterIncludesEscalatedR008F: the ?open=true list must
// include escalated incidents.
func TestIncidentOpenFilterIncludesEscalatedR008F(t *testing.T) {
	deps, srv := newTestDeps(t)
	created, err := deps.Incident.Report(context.Background(), incident.Input{
		SegmentID: "SEG-9", Severity: incident.SeverityWarning, Title: "t", Reporter: "ops",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deps.Incident.Confirm(context.Background(), created.ID, "ops"); err != nil {
		t.Fatal(err)
	}
	if _, err := deps.Incident.Escalate(context.Background(), created.ID, "why"); err != nil {
		t.Fatal(err)
	}
	body := getBody(t, srv, "/api/incidents?open=true", http.StatusOK)
	var list []incident.Incident
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, in := range list {
		if in.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("escalated incident missing from open incident list")
	}
}
