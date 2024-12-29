package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/nsqlite/nsqlite/internal/nsqlite/config"
	"github.com/nsqlite/nsqlite/internal/version"
)

type Client struct {
	httpClient httpClient
}

func NewClient(connStr config.ConnectionString) Client {
	return Client{
		httpClient: newHttpClient(connStr),
	}
}

// IsHealthy checks if the we can connect to the remote server and if
// the server is NSQLite.
func (c *Client) IsHealthy() error {
	res, err := c.httpClient.Get(GetParams{
		Path: "/health",
	})
	if err != nil {
		return err
	}

	if strings.ToLower(res.Body) != "ok" {
		if len(res.Body) > 100 {
			res.Body = res.Body[:100] + "..."
		}
		return fmt.Errorf(
			`health check expected to return "OK" but got "%s"`, res.Body,
		)
	}

	if strings.ToLower(res.Headers.Get("x-server")) != "nsqlite" {
		return fmt.Errorf(
			`health check expected to return "NSQLite" in "X-Server" header but got "%s"`,
			res.Headers.Get("x-server"),
		)
	}

	return nil
}

// RemoteVersion returns the remote NSQLite server version.
//
// The second return value is true when the server is running on different
// version of NSQLite and should show a warning to the user.
func (c *Client) RemoteVersion() (string, bool, error) {
	res, err := c.httpClient.Get(GetParams{
		Path: "/version",
	})
	if err != nil {
		return "", false, fmt.Errorf("failed to get remote NSQLite server version: %w", err)
	}

	if res.Status == http.StatusUnauthorized {
		return "", false, fmt.Errorf("authentication failed, please check your credentials")
	}

	if res.Status != http.StatusOK {
		return "", false, fmt.Errorf("unexpected status code: %d", res.Status)
	}

	isDifferentVersion := res.Body != version.Version
	return res.Body, isDifferentVersion, nil
}

// SendQueryResponse represents the response of a query sent to the remote
// server.
type SendQueryResponse struct {
	Type string  `json:"type"`
	Time float64 `json:"time"`

	// For read queries
	Columns []string `json:"columns"`
	Types   []string `json:"types"`
	Values  [][]any  `json:"values"`

	// For write queries
	LastInsertID int64 `json:"lastInsertId"`
	RowsAffected int64 `json:"rowsAffected"`

	// For begin, commit, and rollback
	TxId string `json:"txId"`

	// For errors
	Error string `json:"error"`
}

// SendQuery sends a query to the remote server and returns the response.
//
// If non empty, txId is used to send the query in the context of a transaction.
func (c *Client) SendQuery(query, txId string) (SendQueryResponse, error) {
	res := SendQueryResponse{}
	body := map[string]string{
		"query": query,
	}
	if txId != "" {
		body["txId"] = txId
	}

	hres, err := c.httpClient.Post(PostParams{
		Path: "/query",
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: body,
	})
	if err != nil {
		return res, fmt.Errorf("failed to send query: %w", err)
	}

	completeRes := struct {
		Results []SendQueryResponse `json:"results"`
	}{}

	if err := json.Unmarshal([]byte(hres.Body), &completeRes); err != nil {
		return res, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(completeRes.Results) == 0 {
		return res, fmt.Errorf("empty response")
	}

	return completeRes.Results[0], nil
}
