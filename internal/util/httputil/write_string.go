package httputil

import (
	"fmt"
	"net/http"
)

// WriteString writes a string response to the given http.ResponseWriter.
func WriteString(w http.ResponseWriter, status int, str string) error {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(status)

	if _, err := w.Write([]byte(str)); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	return nil
}
