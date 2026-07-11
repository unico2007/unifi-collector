// Package respond holds the shared HTTP response helper used by every handler
// package. It is a leaf: it imports nothing from the rest of the BFF, so any
// feature package can depend on it without risking an import cycle.
package respond

import (
	"encoding/json"
	"net/http"
)

// JSON writes v as a JSON body with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
