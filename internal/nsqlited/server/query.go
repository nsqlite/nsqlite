package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nsqlite/nsqlite/internal/nsqlited/db"
	"github.com/nsqlite/nsqlite/internal/util/httputil"
)

// Query represents a single query within a request.
type Query struct {
	TxId   string `json:"txId,omitempty"`
	Query  string `json:"query"`
	Params []any  `json:"params"`
}

// Request represents the structure of an incoming request.
type Request struct {
	TxId    string  `json:"txId,omitempty"`
	Queries []Query `json:"queries"`
}

// WriteResult represents the structure of a write query result.
type WriteResult struct {
	LastInsertID int64   `json:"last_insert_id"`
	RowsAffected int64   `json:"rows_affected"`
	Time         float64 `json:"time"`
}

// ReadResult represents the structure of a read query result.
type ReadResult struct {
	Columns []string `json:"columns"`
	Types   []string `json:"types"`
	Values  [][]any  `json:"values"`
	Time    float64  `json:"time"`
}

// ErrorResult represents the structure of an error result.
type ErrorResult struct {
	Error string  `json:"error"`
	Time  float64 `json:"time"`
}

// Response represents the structure of an outgoing response.
type Response struct {
	Results []any   `json:"results"`
	Error   string  `json:"error,omitempty"`
	Time    float64 `json:"time"`
}

// queryHandler is the HTTP handler for the /query endpoint that
// executes SQL queries.
func (s *Server) queryHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	req := Request{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return httputil.NewJSONError(
			http.StatusBadRequest, err, "Invalid request format",
		)
	}

	allStart := time.Now()
	results := []any{}

	for _, q := range req.Queries {
		txId := req.TxId
		if q.TxId != "" {
			txId = q.TxId
		}

		thisStart := time.Now()
		res, err := s.conf.Db.Query(ctx, db.Query{
			TxId:   txId,
			Query:  q.Query,
			Params: q.Params,
		})
		if err != nil {
			results = append(results, ErrorResult{
				Error: err.Error(),
				Time:  time.Since(thisStart).Seconds(),
			})
			continue
		}

		if res.Type == db.QueryTypeRead {
			results = append(results, ReadResult{
				Columns: res.ReadResult.Columns,
				Types:   res.ReadResult.Types,
				Values:  res.ReadResult.Values,
				Time:    time.Since(thisStart).Seconds(),
			})
			continue
		}

		if res.Type == db.QueryTypeWrite {
			results = append(results, WriteResult{
				LastInsertID: res.WriteResult.LastInsertID,
				RowsAffected: res.WriteResult.RowsAffected,
				Time:         time.Since(thisStart).Seconds(),
			})
			continue
		}
	}

	return httputil.WriteJSON(w, http.StatusOK, Response{
		Results: results,
		Time:    time.Since(allStart).Seconds(),
	})
}
