package platform

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// DecodeJSON decodes r.Body into dst. It rejects bodies that are empty, that
// fail to parse, or that contain trailing non-whitespace data. dst must be a
// non-nil pointer. Returns ErrInvalid on any decode problem.
func DecodeJSON(r *http.Request, dst any) error {
	if r == nil || r.Body == nil {
		return Invalidf("empty request body")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if err == io.EOF {
			return Invalidf("empty request body")
		}
		return Invalidf("malformed JSON: %v", err)
	}
	// ensure no trailing content beyond a single object
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Invalidf("unexpected trailing content in JSON body")
	}
	return nil
}

// DecodeJSONLoose decodes like DecodeJSON but allows unknown fields, for
// endpoints where clients may send extra metadata we wish to ignore.
func DecodeJSONLoose(r *http.Request, dst any) error {
	if r == nil || r.Body == nil {
		return Invalidf("empty request body")
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		if err == io.EOF {
			return Invalidf("empty request body")
		}
		return Invalidf("malformed JSON: %v", err)
	}
	return nil
}

// WriteJSON serializes v as JSON and writes it to w with the given status.
// It sets Content-Type to application/json and includes a newline.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "")
	_ = enc.Encode(v)
}

// WriteJSONPretty serializes v with 2-space indentation, for human-facing
// endpoints such as /api/summary.
func WriteJSONPretty(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// WriteError writes a structured error response. The code is derived from the
// error category; message is the human-readable explanation.
func WriteError(w http.ResponseWriter, status int, code string, message string) {
	if message == "" {
		message = "internal error"
	}
	WriteJSON(w, status, map[string]any{
		"error":   code,
		"message": message,
	})
}

// MarshalCompact returns a compact JSON encoding of v.
func MarshalCompact(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Pretty returns a 2-space-indented JSON encoding of v.
func Pretty(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// ContentTypeJSON reports whether the request declares JSON content.
func ContentTypeJSON(r *http.Request) bool {
	if r == nil {
		return false
	}
	return strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "json")
}

// ReadAll reads and returns the full body, capped at maxBytes.
func ReadAll(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	return io.ReadAll(io.LimitReader(r, maxBytes))
}
