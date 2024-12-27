package server

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryParseRequest(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        []byte
		expected    []Query
		expectErr   bool
	}{
		{
			name:        "Plain text single query",
			contentType: "text/plain",
			body:        []byte("SELECT * FROM table;"),
			expected: []Query{
				{Query: "SELECT * FROM table;"},
			},
		},
		{
			name:        "Plain text empty",
			contentType: "text/plain",
			body:        []byte("  "),
			expectErr:   true,
		},
		{
			name:        "JSON single string",
			contentType: "application/json",
			body:        []byte(`"SELECT * FROM table;"`),
			expected: []Query{
				{Query: "SELECT * FROM table;"},
			},
		},
		{
			name:        "JSON empty string",
			contentType: "application/json",
			body:        []byte(`""`),
			expectErr:   true,
		},
		{
			name:        "JSON array of strings",
			contentType: "application/json",
			body: []byte(`
			[
				"SELECT * FROM table1;",
				"SELECT * FROM table2;"
			]`),
			expected: []Query{
				{Query: "SELECT * FROM table1;"},
				{Query: "SELECT * FROM table2;"},
			},
		},
		{
			name:        "JSON array of objects with params",
			contentType: "application/json",
			body: []byte(`
			[
				{"query": "SELECT * FROM table WHERE id = ?;", "params": [1]},
				{"query": "SELECT * FROM table WHERE id = ? AND foo = ?;", "params": [2, "bar"]}
			]`),
			expected: []Query{
				{Query: "SELECT * FROM table WHERE id = ?;", Params: []any{float64(1)}},
				{Query: "SELECT * FROM table WHERE id = ? AND foo = ?;", Params: []any{float64(2), "bar"}},
			},
		},
		{
			name:        "JSON array of objects with txId",
			contentType: "application/json",
			body: []byte(`
			[
				{"txId": "123", "query": "SELECT 1;", "params": [1]},
				{"txId": "456", "query": "SELECT 2;", "params": [2]}
			]`),
			expected: []Query{
				{TxId: "123", Query: "SELECT 1;", Params: []any{float64(1)}},
				{TxId: "456", Query: "SELECT 2;", Params: []any{float64(2)}},
			},
		},
		{
			name:        "JSON object with top-level txId and queries",
			contentType: "application/json",
			body: []byte(`
			{
				"txId": "999",
				"queries": [
					{"query": "SELECT * FROM table WHERE id = ?;", "params": [10]},
					{"txId": "123", "query": "SELECT 2;", "params": [20]},
					{"query": "SELECT 3;", "params": [30]}
				]
			}`),
			expected: []Query{
				{TxId: "999", Query: "SELECT * FROM table WHERE id = ?;", Params: []any{float64(10)}},
				{TxId: "123", Query: "SELECT 2;", Params: []any{float64(20)}},
				{TxId: "999", Query: "SELECT 3;", Params: []any{float64(30)}},
			},
		},
		{
			name:        "JSON object single query",
			contentType: "application/json",
			body: []byte(`
			{
				"txId": "123",
				"query": "SELECT 1;",
				"params": [1]
			}`),
			expected: []Query{
				{TxId: "123", Query: "SELECT 1;", Params: []any{float64(1)}},
			},
		},
		{
			name:        "JSON invalid structure",
			contentType: "application/json",
			body:        []byte("123"),
			expectErr:   true,
		},
		{
			name:        "JSON invalid array item",
			contentType: "application/json",
			body: []byte(`
			[
				"SELECT 1;",
				123
			]`),
			expectErr: true,
		},
		{
			name:        "Invalid JSON",
			contentType: "application/json",
			body:        []byte(`{"query": "test"`),
			expectErr:   true,
		},
	}

	for idx, testCase := range tests {
		t.Run(fmt.Sprintf("%d: %s", idx+1, testCase.name), func(t *testing.T) {
			queries, err := queryParseRequest(testCase.contentType, testCase.body)
			if testCase.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, queries)
		})
	}
}
