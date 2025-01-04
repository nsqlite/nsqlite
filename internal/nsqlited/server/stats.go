package server

import (
	"net/http"

	"github.com/nsqlite/nsqlite/internal/util/httputil"
)

func (s *Server) statsHandler(w http.ResponseWriter, r *http.Request) error {
	stats, err := s.DBStats.MarshalJSON()
	if err != nil {
		return httputil.NewJSONError(
			http.StatusBadRequest, err, "Failed to collect DB stats, "+err.Error(),
		)
	}

	return httputil.WriteJSONBytes(w, http.StatusOK, stats)
}
