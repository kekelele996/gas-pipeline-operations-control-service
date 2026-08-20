package httpapi

import (
	"context"
	"net/http"
	"testing"
)

// TestNetworkLimitHTTPR004F: recording a segment limit through the API under
// the default config must succeed (200), not panic into a 500.
func TestNetworkLimitHTTPR004F(t *testing.T) {
	deps, srv := newTestDeps(t)
	segs := deps.Network.ListSegments(context.Background())
	if len(segs) == 0 {
		t.Fatal("no seeded segments")
	}
	postJSON(t, srv, "/api/segments/"+segs[0].ID+"/limits", map[string]float64{"value": 6.0}, http.StatusOK)
}
