package server

import (
	"net/http"

	"github.com/nsqlite/nsqlite/internal/util/httputil"
)

type statsResponse struct {
	TotalReadQueries     int64 `json:"totalReadQueries"`
	TotalWriteQueries    int64 `json:"totalWriteQueries"`
	TotalBeginQueries    int64 `json:"totalBeginQueries"`
	TotalCommitQueries   int64 `json:"totalCommitQueries"`
	TotalRollbackQueries int64 `json:"totalRollbackQueries"`
	TransactionsInFlight int64 `json:"transactionsInFlight"`
	WriteQueueLength     int64 `json:"writeQueueLength"`
}

func (s *Server) statsHandler(w http.ResponseWriter, r *http.Request) error {
	stats := s.Db.GetStats()
	resp := statsResponse{
		TotalReadQueries:     stats.TotalReadQueries,
		TotalWriteQueries:    stats.TotalWriteQueries,
		TotalBeginQueries:    stats.TotalBeginQueries,
		TotalCommitQueries:   stats.TotalCommitQueries,
		TotalRollbackQueries: stats.TotalRollbackQueries,
		TransactionsInFlight: stats.TransactionsInFlight,
		WriteQueueLength:     stats.WriteQueueLength,
	}

	return httputil.WriteJSON(w, http.StatusOK, resp)
}
