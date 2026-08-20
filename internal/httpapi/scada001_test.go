package httpapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"gas-pipeline-operations-control-service/internal/scada"
)

// TestScadaHistoryHTTPStableR001D: the history endpoint must return the
// actually stored readings, not stale ring slots, even with a generous limit.
func TestScadaHistoryHTTPStableR001D(t *testing.T) {
	deps, srv := newTestDeps(t)
	deps.SCADA.RegisterPoint(context.Background(), scada.Point{
		ID: "PT-HTTP", Name: "p", Type: scada.TypePressure, Enabled: true,
		HighLimit: 10, LowLimit: 0, PhysicalMin: -5, PhysicalMax: 50,
	})
	now := time.Now()
	for i := 0; i < 10; i++ {
		deps.SCADA.Ingest(context.Background(), []scada.Reading{{
			PointID: "PT-HTTP", Value: 1.0 + float64(i%7), Ts: now.Add(time.Duration(i) * time.Second),
		}})
	}
	body := getBody(t, srv, "/api/scada/points/PT-HTTP/history?limit=100", 200)
	var got []scada.Reading
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal history: %v", err)
	}
	if len(got) != 10 {
		t.Fatalf("history length = %d, want 10", len(got))
	}
	for i, rd := range got {
		if rd.Value == 0 {
			t.Fatalf("history[%d] is a stale zero reading", i)
		}
	}
}
