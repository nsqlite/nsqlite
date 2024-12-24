package server

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/nsqlite/nsqlite/internal/log"
	"github.com/nsqlite/nsqlite/internal/util/httputil"
)

func (s *Server) errorHandler(
	w http.ResponseWriter, r *http.Request, err error,
) {
	ip := httputil.ReadUserIP(r)
	errorURL := r.URL.String()
	errorId := uuid.NewString()

	switch err := err.(type) {
	case httputil.JSONError:
		statusText := http.StatusText(err.HTTPStatus)
		safeMessage := err.SafeMessage
		if safeMessage == "" {
			safeMessage = statusText
		}

		// Log the real error to the server logger.
		s.conf.Logger.ErrorNs(
			log.NsServer, "error while handling request", log.KV{
				"id":      errorId,
				"status":  err.HTTPStatus,
				"error":   err.Error(),
				"message": safeMessage,
				"url":     errorURL,
				"ip":      ip,
			},
		)

		// Respond with a safe error message.
		_ = httputil.WriteJSON(w, err.HTTPStatus, map[string]any{
			"id":      errorId,
			"error":   statusText,
			"message": safeMessage,
		})
	default:
		s.conf.Logger.ErrorNs(
			log.NsServer, "unknown error while handling request", log.KV{
				"id":    errorId,
				"error": err.Error(),
				"url":   errorURL,
				"ip":    ip,
			},
		)
		_ = httputil.WriteString(
			w, http.StatusInternalServerError, "Internal Server Error - "+errorId,
		)
	}
}
