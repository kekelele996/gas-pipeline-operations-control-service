package httpapi

import (
	"net/http"
	"testing"
)

// TestMeteringSettlementHTTPStatusR003D: a missing settlement must be a 404,
// not a 200-with-empty-draft.
func TestMeteringSettlementHTTPStatusR003D(t *testing.T) {
	_, srv := newTestDeps(t)
	getBody(t, srv, "/api/metering/settlements/2030-01-01", http.StatusNotFound)
}

// TestMeteringConfirmHTTPStatusR003E: confirming a missing settlement must be
// a 404, not a swallowed 200.
func TestMeteringConfirmHTTPStatusR003E(t *testing.T) {
	_, srv := newTestDeps(t)
	postJSON(t, srv, "/api/metering/settle/2030-01-01/confirm", map[string]string{"by": "ops"}, http.StatusNotFound)
}

// TestMeteringReconcileHTTPStatusR003H: reconciling a missing settlement must
// be a 404, not a swallowed 200.
func TestMeteringReconcileHTTPStatusR003H(t *testing.T) {
	_, srv := newTestDeps(t)
	postJSON(t, srv, "/api/metering/settle/2030-01-01/reconcile", map[string]string{"by": "ops"}, http.StatusNotFound)
}

// TestMeteringSettleEmptyDateHTTPStatusR003I: an empty settle date must be a
// 400, not a 500.
func TestMeteringSettleEmptyDateHTTPStatusR003I(t *testing.T) {
	_, srv := newTestDeps(t)
	postJSON(t, srv, "/api/metering/settle", map[string]string{"date": ""}, http.StatusBadRequest)
}
