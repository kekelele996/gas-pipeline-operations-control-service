package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gas-pipeline-operations-control-service/internal/dispatch"
	"gas-pipeline-operations-control-service/internal/network"
)

// TestDispatchExecuteHTTPCancelR005D: executing an order with a cancelled
// request context must not succeed.
func TestDispatchExecuteHTTPCancelR005D(t *testing.T) {
	deps, _ := newTestDeps(t)
	segs := deps.Network.ListSegments(context.Background())
	if len(segs) == 0 {
		t.Fatal("no seeded segments")
	}
	if _, err := deps.Network.UpsertValve(context.Background(), network.ValveInput{
		ID: "V-EXEC", Name: "exec", SegmentID: segs[0].ID, Type: "block",
		State: network.ValveClosed, Normal: "open", RemoteControllable: true,
	}); err != nil {
		t.Fatal(err)
	}

	created, err := deps.Dispatch.Create(context.Background(), dispatch.Input{
		Type: dispatch.TypeOpenValve, TargetID: "V-EXEC", Reason: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deps.Dispatch.Issue(context.Background(), created.ID, "ops"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/orders/"+created.ID+"/execute", nil)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	NewRouter(deps).ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatal("execute succeeded with a cancelled request context")
	}
}
