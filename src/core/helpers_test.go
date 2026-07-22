package core

import (
	"encoding/json"
	"net/http"
)

// clientFor builds a client pointed at a test server with a fixed device token.
func clientFor(url string) *Client {
	return NewClient(Config{APIBase: url, DeviceToken: "dev-token"})
}

// writeJSON writes a status code and optional JSON body in a test handler.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// decodeBody decodes a request body into v in a test handler.
func decodeBody(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
