package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nsqlite/nsqlite/internal/nsqlited/db"
	"github.com/nsqlite/nsqlite/internal/util/httputil"
	"github.com/nsqlite/nsqlite/internal/validate"
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

// queryHandler is the HTTP handler for the /query endpoint that
// executes SQL queries.
func (s *Server) queryHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	contentType := r.Header.Get("Content-Type")
	isContentTypeValid := validate.ContentType(
		contentType, validate.ContentTypeJSON, validate.ContentTypePlainText,
	)
	if !isContentTypeValid {
		return httputil.NewJSONError(
			http.StatusBadRequest, nil, "Invalid content type",
		)
	}

	body, err := httputil.ReadReqBodyBytes(r)
	if err != nil {
		return httputil.NewJSONError(
			http.StatusBadRequest, err, "Failed to read request body",
		)
	}

	queries, err := queryParseRequest(contentType, body)
	if err != nil {
		return httputil.NewJSONError(
			http.StatusBadRequest, err, "Failed to parse request body, "+err.Error(),
		)
	}

	allStart := time.Now()
	results := []any{}

	for _, q := range queries {
		thisStart := time.Now()
		res, err := s.Db.Query(ctx, db.Query{
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
