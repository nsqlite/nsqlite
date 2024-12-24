package httputil

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// WriteJSON writes a JSON response to the given http.ResponseWriter.
func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("failed to write JSON response: %w", err)
	}

	return nil
}
