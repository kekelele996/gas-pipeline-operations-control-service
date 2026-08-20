package httpapi

// helpers.go: small request/response conveniences shared by handler files.

import (
	"net/http"
	"strconv"

	"gas-pipeline-operations-control-service/internal/platform"
)

// pathValue reads a path parameter from the request (Go 1.22+ ServeMux).
func pathValue(r *http.Request, key string) string {
	return r.PathValue(key)
}

// queryInt reads an integer query parameter with a default.
func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// queryBool reads a boolean query parameter, default false.
func queryBool(r *http.Request, key string) bool {
	v := r.URL.Query().Get(key)
	return v == "true" || v == "1" || v == "yes"
}

// decodeBody decodes a JSON request body into dst, mapping decode errors.
func decodeBody(r *http.Request, dst any) error {
	return platform.DecodeJSONLoose(r, dst)
}

// writeJSON writes a JSON response with the standard helper.
func writeJSON(w http.ResponseWriter, status int, v any) {
	platform.WriteJSON(w, status, v)
}

// noContent writes 204.
func noContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
