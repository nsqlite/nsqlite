package server

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/nsqlite/nsqlite/internal/validate"
)

// Query represents a single query within a request.
type Query struct {
	TxId   string `json:"txId,omitempty"`
	Query  string `json:"query"`
	Params []any  `json:"params"`
}

// queryParseRequest parses the request body into a slice of Query objects.
// It supports plain text queries, JSON arrays (strings or objects), and nested
// structures.
//
// The function adapts its behavior based on the Content-Type header, ensuring
// flexible and robust query parsing for various input formats.
//
// Look at the tests for examples of supported input formats.
func queryParseRequest(contentType string, body []byte) ([]Query, error) {
	isJSON := validate.ContentType(contentType, validate.ContentTypeJSON)
	if !isJSON {
		trimmedQuery := strings.TrimSpace(string(body))
		if trimmedQuery == "" {
			return nil, errors.New("empty query")
		}
		return []Query{{Query: trimmedQuery}}, nil
	}

	var rawBody any
	if err := json.Unmarshal(body, &rawBody); err != nil {
		return nil, err
	}

	switch content := rawBody.(type) {
	case string:
		trimmedQuery := strings.TrimSpace(content)
		if trimmedQuery == "" {
			return nil, errors.New("empty query")
		}
		return []Query{{Query: trimmedQuery}}, nil

	case []any:
		resultQueries := make([]Query, 0, len(content))
		for _, element := range content {
			switch queryElement := element.(type) {
			case string:
				trimmedQuery := strings.TrimSpace(queryElement)
				if trimmedQuery == "" {
					return nil, errors.New("empty query")
				}
				resultQueries = append(resultQueries, Query{Query: trimmedQuery})
			case map[string]any:
				newQuery := Query{}
				if queryStr, ok := queryElement["query"].(string); ok {
					newQuery.Query = strings.TrimSpace(queryStr)
				}
				if params, ok := queryElement["params"].([]any); ok {
					newQuery.Params = params
				}
				if txID, ok := queryElement["txId"].(string); ok {
					newQuery.TxId = txID
				}
				if newQuery.Query == "" {
					return nil, errors.New("empty query")
				}
				resultQueries = append(resultQueries, newQuery)
			default:
				return nil, errors.New("invalid array item")
			}
		}
		return resultQueries, nil

	case map[string]any:
		topLevelTxID, _ := content["txId"].(string)
		if queriesData, ok := content["queries"].([]any); ok {
			resultQueries := make([]Query, 0, len(queriesData))
			for _, item := range queriesData {
				queryMap, isMap := item.(map[string]any)
				if !isMap {
					return nil, errors.New("invalid query object")
				}
				newQuery := Query{}
				if queryStr, ok := queryMap["query"].(string); ok {
					newQuery.Query = strings.TrimSpace(queryStr)
				}
				if params, ok := queryMap["params"].([]any); ok {
					newQuery.Params = params
				}
				if txID, ok := queryMap["txId"].(string); ok {
					newQuery.TxId = txID
				} else {
					newQuery.TxId = topLevelTxID
				}
				if newQuery.Query == "" {
					return nil, errors.New("empty query")
				}
				resultQueries = append(resultQueries, newQuery)
			}
			return resultQueries, nil
		}

		newQuery := Query{TxId: topLevelTxID}
		if queryStr, ok := content["query"].(string); ok {
			newQuery.Query = strings.TrimSpace(queryStr)
		}
		if params, ok := content["params"].([]any); ok {
			newQuery.Params = params
		}
		if newQuery.Query == "" {
			return nil, errors.New("no valid query found")
		}
		return []Query{newQuery}, nil

	default:
		return nil, errors.New("unsupported JSON structure")
	}
}
