package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nsqlite/nsqlite/internal/nsqlited/db"
	"github.com/nsqlite/nsqlite/internal/nsqlited/sqlitec"
	"github.com/nsqlite/nsqlite/internal/util/httputil"
)

// ResponseResult represents the structure of a query result.
type ResponseResult struct {
	Time  float64 `json:"time"`
	TxId  string  `json:"txId,omitempty"`
	Error string  `json:"error,omitempty"`

	LastInsertID int64 `json:"lastInsertId,omitempty"`
	RowsAffected int64 `json:"rowsAffected,omitempty"`

	Columns []string `json:"columns,omitempty"`
	Types   []string `json:"types,omitempty"`
	Rows    [][]any  `json:"rows,omitempty"`
}

// Response represents the structure of an outgoing response.
type Response struct {
	Time    float64          `json:"time"`
	Results []ResponseResult `json:"results"`
}

// Query represents a single query within a request.
type Query struct {
	TxId   string               `json:"txId"`
	Query  string               `json:"query"`
	Params []sqlitec.QueryParam `json:"params"`
}

// queryHandler is the HTTP handler for the /query endpoint that
// executes SQL queries.
func (s *Server) queryHandler(w http.ResponseWriter, r *http.Request) error {
	s.DBStats.IncHTTPRequests()
	s.DBStats.IncQueuedHTTPRequests()
	defer s.DBStats.DecQueuedHTTPRequests()
	ctx := r.Context()

	var queries []Query
	if err := json.NewDecoder(r.Body).Decode(&queries); err != nil {
		return httputil.NewJSONError(
			http.StatusBadRequest, err, "Failed to read request body",
		)
	}

	allStart := time.Now()
	results := []ResponseResult{}

	for _, q := range queries {
		thisStart := time.Now()

		if q.Query == "" {
			results = append(results, ResponseResult{
				Time:  time.Since(thisStart).Seconds(),
				Error: "Empty query",
			})
			continue
		}

		res, err := s.DB.Query(ctx, db.Query{
			TxId:   q.TxId,
			Query:  q.Query,
			Params: q.Params,
		})
		if err != nil {
			results = append(results, ResponseResult{
				Time:  time.Since(thisStart).Seconds(),
				Error: err.Error(),
			})
			continue
		}

		results = append(results, ResponseResult{
			Time: time.Since(thisStart).Seconds(),
			TxId: res.TxId,

			LastInsertID: res.LastInsertID,
			RowsAffected: res.RowsAffected,

			Columns: res.Columns,
			Types:   res.Types,
			Rows:    res.Rows,
		})
	}

	return httputil.WriteJSON(w, http.StatusOK, Response{
		Time:    time.Since(allStart).Seconds(),
		Results: results,
	})
}
