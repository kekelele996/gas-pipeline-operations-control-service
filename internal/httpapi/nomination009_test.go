package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/contract"
	"gas-pipeline-operations-control-service/internal/nomination"
)

// TestNominationOverCapacityHTTPStatusR009C: submitting a nomination beyond
// capacity through the API must be a 409, not a swallowed 200 or a 500.
func TestNominationOverCapacityHTTPStatusR009C(t *testing.T) {
	deps, srv := newTestDeps(t)
	now := time.Now()
	c, err := deps.Contract.Create(context.Background(), contract.ContractInput{
		ShipperID: "SH", ShipperName: "s", Code: "C-HTTP", DailyVolume: 100,
		ValidFrom: now.Add(-time.Hour), ValidTo: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	created, err := deps.Nomination.Create(context.Background(), nomination.Input{
		ContractID: c.ID, Date: "2026-09-02", Volume: 150,
	})
	if err != nil {
		t.Fatal(err)
	}
	postJSON(t, srv, "/api/nominations/"+created.ID+"/submit", map[string]string{"by": "shipper"}, http.StatusConflict)
}
