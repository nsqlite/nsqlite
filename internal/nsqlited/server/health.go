package server

import (
	"net/http"

	"github.com/nsqlite/nsqlite/internal/nsqlited/db"
	"github.com/nsqlite/nsqlite/internal/util/httputil"
)

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) error {
	queryRes, err := s.conf.Db.Query(r.Context(), db.Query{
		Query: "SELECT 1",
	})
	if err != nil {
		return httputil.NewJSONError(
			http.StatusInternalServerError, err, "Failed to query the database",
		)
	}
	if queryRes.Type == db.QueryTypeRead {
		defer queryRes.ReadResult.Close()
	}

	return httputil.WriteString(w, http.StatusOK, "OK")
}
