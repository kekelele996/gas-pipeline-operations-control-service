package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/permit"
)

// TestPermitApproveHTTPConflictR007F: approving an overlapping permit through
// the API must return 409, not a fake 200.
func TestPermitApproveHTTPConflictR007F(t *testing.T) {
	deps, srv := newTestDeps(t)
	base := time.Now()
	a := postPermit(t, deps, srv, "SEG-7", base, base.Add(2*time.Hour))
	postJSON(t, srv, "/api/permits/"+a+"/approve", map[string]string{"approver": "mgr"}, http.StatusOK)
	b := postPermit(t, deps, srv, "SEG-7", base.Add(time.Hour), base.Add(3*time.Hour))
	postJSON(t, srv, "/api/permits/"+b+"/approve", map[string]string{"approver": "mgr"}, http.StatusConflict)
}

func postPermit(t *testing.T, deps Deps, srv interface{}, seg string, start, end time.Time) string {
	t.Helper()
	created, err := deps.Permit.Apply(context.Background(), permit.Input{
		SegmentID: seg, WorkType: "inspection", Title: "t", Applicant: "ops",
		WindowStart: start, WindowEnd: end,
	})
	if err != nil {
		t.Fatal(err)
	}
	return created.ID
}
