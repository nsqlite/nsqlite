package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nsqlite/nsqlite/internal/nsqlited/db"
	"github.com/nsqlite/nsqlite/internal/util/httputil"
	"github.com/orsinium-labs/enum"
)

type resultType enum.Member[string]

func (r resultType) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, r.Value)), nil
}

var (
	resultTypeWrite    = resultType{"write"}
	resultTypeRead     = resultType{"read"}
	resultTypeBegin    = resultType{"begin"}
	resultTypeCommit   = resultType{"commit"}
	resultTypeRollback = resultType{"rollback"}
	resultTypeOk       = resultType{"ok"}
	resultTypeError    = resultType{"error"}
)

// WriteResult represents the structure of a write query result.
type WriteResult struct {
	Type         resultType `json:"type"`
	LastInsertID int64      `json:"lastInsertId"`
	RowsAffected int64      `json:"rowsAffected"`
	Time         float64    `json:"time"`
}

// ReadResult represents the structure of a read query result.
type ReadResult struct {
	Type    resultType `json:"type"`
	Columns []string   `json:"columns"`
	Types   []string   `json:"types"`
	Values  [][]any    `json:"values"`
	Time    float64    `json:"time"`
}

// TxResult represents the structure of a transaction operation result.
type TxResult struct {
	Type resultType `json:"type"`
	TxId string     `json:"txId"`
	Time float64    `json:"time"`
}

// SuccessResult represents a generic success result.
type SuccessResult struct {
	Type resultType `json:"type"`
	Time float64    `json:"time"`
}

// ErrorResult represents the structure of an error result.
type ErrorResult struct {
	Type  resultType `json:"type"`
	Error string     `json:"error"`
	Time  float64    `json:"time"`
}

// Response represents the structure of an outgoing response.
type Response struct {
	Results []any   `json:"results"`
	Time    float64 `json:"time"`
}

// Query represents a single query within a request.
type Query struct {
	TxId   string `json:"txId,omitempty"`
	Query  string `json:"query"`
	Params []any  `json:"params"`
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
	results := []any{}

	for _, q := range queries {
		thisStart := time.Now()

		if q.Query == "" {
			results = append(results, ErrorResult{
				Type:  resultTypeError,
				Error: "Empty query",
				Time:  time.Since(thisStart).Seconds(),
			})
			continue
		}

		res, err := s.DB.Query(ctx, db.Query{
			TxId:   q.TxId,
			Query:  q.Query,
			Params: q.Params,
		})
		if err != nil {
			results = append(results, ErrorResult{
				Type:  resultTypeError,
				Error: err.Error(),
				Time:  time.Since(thisStart).Seconds(),
			})
			continue
		}

		if res.Type == db.QueryTypeRead {
			if res.ReadResult.Values == nil {
				results = append(results, ErrorResult{
					Type:  resultTypeError,
					Error: "No rows returned",
					Time:  time.Since(thisStart).Seconds(),
				})
				continue
			}

			results = append(results, ReadResult{
				Type:    resultTypeRead,
				Columns: res.ReadResult.Columns,
				Types:   res.ReadResult.Types,
				Values:  *res.ReadResult.Values,
				Time:    time.Since(thisStart).Seconds(),
			})
			continue
		}

		if res.Type == db.QueryTypeWrite {
			results = append(results, WriteResult{
				Type:         resultTypeWrite,
				LastInsertID: res.WriteResult.LastInsertID,
				RowsAffected: res.WriteResult.RowsAffected,
				Time:         time.Since(thisStart).Seconds(),
			})
			continue
		}

		if res.Type == db.QueryTypeBegin {
			results = append(results, TxResult{
				Type: resultTypeBegin,
				TxId: res.TxId,
				Time: time.Since(thisStart).Seconds(),
			})
			continue
		}

		if res.Type == db.QueryTypeCommit {
			results = append(results, TxResult{
				Type: resultTypeCommit,
				TxId: res.TxId,
				Time: time.Since(thisStart).Seconds(),
			})
			continue
		}

		if res.Type == db.QueryTypeRollback {
			results = append(results, TxResult{
				Type: resultTypeRollback,
				TxId: res.TxId,
				Time: time.Since(thisStart).Seconds(),
			})
			continue
		}

		results = append(results, SuccessResult{
			Type: resultTypeOk,
			Time: time.Since(thisStart).Seconds(),
		})
	}

	return httputil.WriteJSON(w, http.StatusOK, Response{
		Results: results,
		Time:    time.Since(allStart).Seconds(),
	})
}
