package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/audit"
	"gas-pipeline-operations-control-service/internal/contract"
	"gas-pipeline-operations-control-service/internal/dispatch"
	"gas-pipeline-operations-control-service/internal/incident"
	"gas-pipeline-operations-control-service/internal/leakdetect"
	"gas-pipeline-operations-control-service/internal/metering"
	"gas-pipeline-operations-control-service/internal/network"
	"gas-pipeline-operations-control-service/internal/nomination"
	"gas-pipeline-operations-control-service/internal/notify"
	"gas-pipeline-operations-control-service/internal/permit"
	"gas-pipeline-operations-control-service/internal/scada"
	"gas-pipeline-operations-control-service/internal/seed"
)

// testClock implements platform.Clock using the real wall clock (tests don't
// depend on advancing time; they use real now).
type testClock struct{}

func (testClock) Now() time.Time { return time.Now() }

// testAlarmResolver adapts the scada service to incident.AlarmResolver.
type testAlarmResolver struct{ s *scada.Service }

func (a testAlarmResolver) ResolveAlarm(ctx context.Context, id string) error {
	_, err := a.s.ResolveAlarm(ctx, id)
	return err
}

// testLeakSink forwards a leak alert into the notify queue.
type testLeakSink struct{ n *notify.Service }

func (s testLeakSink) OnLeak(ctx context.Context, a leakdetect.LeakAlert) error {
	_, err := s.n.Enqueue(ctx, "oncall", notify.ChannelSMS, "LEAK "+string(a.Severity), a.Message)
	return err
}

// newTestDeps wires real services with a fake clock and seeded data.
func newTestDeps(t *testing.T) (Deps, *httptest.Server) {
	t.Helper()
	c := testClock{}
	auditSvc := audit.NewService(audit.NewStore(1000), c)
	ns := network.NewService(network.NewStore(), c, auditSvc)
	scadaSvc := scada.NewService(scada.NewStore(200), c, 0.5)
	meterSvc := metering.NewService(metering.NewStore(5000), c, auditSvc)
	contractSvc := contract.NewService(contract.NewStore(), c, auditSvc)
	nominationSvc := nomination.NewService(nomination.NewStore(), contractSvc, c, auditSvc)
	incidentSvc := incident.NewService(incident.NewStore(), c, auditSvc, testAlarmResolver{scadaSvc})
	dispatchSvc := dispatch.NewService(dispatch.NewStore(), ns, c, auditSvc)
	permitSvc := permit.NewService(permit.NewStore(), c, dispatchSvc, incidentSvc, auditSvc)
	leakSvc := leakdetect.NewService(c, leakdetect.Thresholds{DropRate: 0.15, Imbalance: 0.05, WindowMinutes: 30})
	notifySvc := notify.NewService(notify.NewStore(), c, notify.Config{MaxAttempts: 3, Backoff: time.Second})
	scadaSvc.AddSink(notifySvc)
	leakSvc.AddSink(testLeakSink{notifySvc})

	seed.All(context.Background(), ns, scadaSvc, meterSvc, contractSvc)

	deps := Deps{
		Network: ns, SCADA: scadaSvc, Metering: meterSvc, Contract: contractSvc,
		Nomination: nominationSvc, Permit: permitSvc, Dispatch: dispatchSvc,
		Incident: incidentSvc, Leak: leakSvc, Audit: auditSvc, Notify: notifySvc,
	}
	srv := httptest.NewServer(NewRouter(deps))
	t.Cleanup(srv.Close)
	return deps, srv
}

func getBody(t *testing.T, srv *httptest.Server, path string, status int) []byte {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != status {
		t.Fatalf("GET %s: status %d, want %d", path, resp.StatusCode, status)
	}
	b, _ := io.ReadAll(resp.Body)
	return b
}

func postJSON(t *testing.T, srv *httptest.Server, path string, body any, status int) []byte {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(srv.URL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != status {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST %s: status %d, want %d; body=%s", path, resp.StatusCode, status, out)
	}
	out, _ := io.ReadAll(resp.Body)
	return out
}

func TestHealthEndpoint(t *testing.T) {
	_, srv := newTestDeps(t)
	b := getBody(t, srv, "/health", 200)
	if !strings.Contains(string(b), `"status":"ok"`) {
		t.Fatalf("health body = %s", b)
	}
}

func TestSummaryEndpoint(t *testing.T) {
	_, srv := newTestDeps(t)
	b := getBody(t, srv, "/api/summary", 200)
	var sum summaryResponse
	if err := json.Unmarshal(b, &sum); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sum.Network.Segments != 4 {
		t.Fatalf("segments = %d, want 4", sum.Network.Segments)
	}
	if sum.Network.Stations != 5 {
		t.Fatalf("stations = %d, want 5", sum.Network.Stations)
	}
}

func TestTelemetryIngestAndAlarms(t *testing.T) {
	_, srv := newTestDeps(t)
	// list points to find a pressure point
	pb := getBody(t, srv, "/api/scada/points", 200)
	var pts []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	json.Unmarshal(pb, &pts)
	var pid string
	for _, p := range pts {
		if p.Type == "pressure" {
			pid = p.ID
			break
		}
	}
	if pid == "" {
		t.Fatal("no pressure point")
	}
	// ingest an out-of-limit reading to raise an alarm
	postJSON(t, srv, "/api/telemetry", map[string]any{
		"readings": []map[string]any{
			{"point_id": pid, "value": 5.0, "ts": "2026-01-01T00:00:00Z"},
			{"point_id": pid, "value": 9.6, "ts": "2026-01-01T00:00:01Z"},
		},
	}, 200)
	// alarms should now contain one
	ab := getBody(t, srv, "/api/alarms?active=true", 200)
	if !strings.Contains(string(ab), pid) {
		t.Fatalf("expected alarm for %s in %s", pid, ab)
	}
}

func TestNominationSubmitViaAPI(t *testing.T) {
	deps, srv := newTestDeps(t)
	// get the seeded contract
	cb := getBody(t, srv, "/api/contracts", 200)
	var contracts []struct {
		ID string `json:"id"`
	}
	json.Unmarshal(cb, &contracts)
	if len(contracts) == 0 {
		t.Fatal("no contract")
	}
	cid := contracts[0].ID
	// create + submit a nomination
	nb := postJSON(t, srv, "/api/nominations", map[string]any{
		"contract_id": cid, "date": "2026-08-20", "volume": 100000, "submitted_by": "api",
	}, 201)
	var n struct {
		ID string `json:"id"`
	}
	json.Unmarshal(nb, &n)
	postJSON(t, srv, "/api/nominations/"+n.ID+"/submit", map[string]any{"by": "api"}, 200)
	// remaining should drop
	rb := getBody(t, srv, "/api/contracts/"+cid+"/remaining", 200)
	if !strings.Contains(string(rb), "300000") {
		t.Fatalf("remaining after submit = %s", rb)
	}
	_ = deps
}

func TestErrorMappingNotFound(t *testing.T) {
	_, srv := newTestDeps(t)
	resp, err := http.Get(srv.URL + "/api/segments/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}
