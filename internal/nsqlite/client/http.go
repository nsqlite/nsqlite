package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/nsqlite/nsqlite/internal/nsqlite/config"
)

type httpClient struct {
	connStr    config.ConnectionString
	httpClient *http.Client
}

func newHttpClient(connStr config.ConnectionString) httpClient {
	return httpClient{
		connStr: connStr,
		httpClient: &http.Client{
			Timeout: 0,
		},
	}
}

type createRequestParams struct {
	method string
	path   string
	header http.Header
}

func (hc *httpClient) createRequest(
	params createRequestParams,
) (*http.Request, error) {
	if params.method == "" {
		params.method = http.MethodGet
	}
	if params.path == "" {
		params.path = "/"
	}
	if params.header == nil {
		params.header = http.Header{}
	}
	if params.header.Get("Content-Type") == "" {
		params.header.Set("Content-Type", "application/json")
	}

	joinedUrl, err := url.JoinPath(hc.connStr.URL(), params.path)
	if err != nil {
		return nil, fmt.Errorf("failed to create request URL: %w", err)
	}

	parsedUrl, err := url.Parse(joinedUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse request URL: %w", err)
	}

	params.header.Set("Authorization", hc.connStr.AuthToken())
	req := &http.Request{
		Method: params.method,
		URL:    parsedUrl,
		Header: params.header,
	}

	return req, nil
}

// GetParams represents the parameters for the Get method.
type GetParams struct {
	Path   string
	Header http.Header
}

// GetResponse represents the response from Get.
type GetResponse struct {
	IsJson     bool
	Body       string
	Status     int
	StatusText string
	Headers    http.Header
}

// GetText sends a GET request to specified path.
func (hc *httpClient) Get(params GetParams) (GetResponse, error) {
	res := GetResponse{}

	req, err := hc.createRequest(createRequestParams{
		method: http.MethodGet,
		path:   params.Path,
		header: params.Header,
	})
	if err != nil {
		return res, err
	}

	hres, err := hc.httpClient.Do(req)
	if err != nil {
		return res, fmt.Errorf("failed sending GET request: %w", err)
	}
	defer hres.Body.Close()

	bodyb, err := io.ReadAll(hres.Body)
	if err != nil {
		return res, fmt.Errorf("failed reading response body: %w", err)
	}

	isJson := strings.Contains(hres.Header.Get("Content-Type"), "application/json")
	res = GetResponse{
		IsJson:     isJson,
		Body:       string(bodyb),
		Status:     hres.StatusCode,
		StatusText: hres.Status,
		Headers:    hres.Header,
	}
	return res, nil
}

// PostParams represents the parameters for the Post method.
type PostParams struct {
	Path   string
	Body   any
	Header http.Header
}

// PostResponse represents the response from Post.
type PostResponse struct {
	IsJson     bool
	Body       string
	Status     int
	StatusText string
	Headers    http.Header
}

// Post sends a POST request to specified path.
func (hc *httpClient) Post(params PostParams) (PostResponse, error) {
	body := []byte{}

	if params.Body != nil {
		switch v := params.Body.(type) {
		case string:
			body = []byte(v)
		case map[string]string:
			b, err := json.Marshal(v)
			if err != nil {
				return PostResponse{}, fmt.Errorf("failed to marshal body: %w", err)
			}
			body = b
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return PostResponse{}, fmt.Errorf("failed to marshal body: %w", err)
			}
			body = b
		}
	}

	res := PostResponse{}

	req, err := hc.createRequest(createRequestParams{
		method: http.MethodPost,
		path:   params.Path,
		header: params.Header,
	})
	if err != nil {
		return res, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))

	hres, err := hc.httpClient.Do(req)
	if err != nil {
		return res, fmt.Errorf("failed sending POST request: %w", err)
	}
	defer hres.Body.Close()

	bodyb, err := io.ReadAll(hres.Body)
	if err != nil {
		return res, fmt.Errorf("failed reading response body: %w", err)
	}

	isJson := strings.Contains(hres.Header.Get("Content-Type"), "application/json")
	res = PostResponse{
		IsJson:     isJson,
		Body:       string(bodyb),
		Status:     hres.StatusCode,
		StatusText: hres.Status,
		Headers:    hres.Header,
	}
	return res, nil
}
