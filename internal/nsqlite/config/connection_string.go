package config

import (
	"errors"
	"net/url"
)

// ConnectionString represents the connection string for the NSQLite
// database server.
type ConnectionString struct {
	protocol  string
	host      string
	port      string
	authToken string
}

// String returns the string representation of the connection string without
// the auth token.
func (c ConnectionString) String() string {
	if c.authToken == "" {
		return c.protocol + "://" + c.host + ":" + c.port
	}

	return c.protocol + "://" + c.host + ":" + c.port + "?authToken=****"
}

// URL returns the URL of the connection string without the auth token.
func (c ConnectionString) URL() string {
	return c.protocol + "://" + c.host + ":" + c.port
}

// AuthToken returns the auth token of the connection string.
func (c ConnectionString) AuthToken() string {
	return c.authToken
}

// parseConnectionString parses the given connection string and returns
// a ConnectionString struct.
func parseConnectionString(connectionString string) (ConnectionString, error) {
	parsedURL, err := url.Parse(connectionString)
	if err != nil {
		return ConnectionString{}, err
	}

	protocol := parsedURL.Scheme
	if protocol != "http" && protocol != "https" {
		return ConnectionString{}, errors.New("invalid protocol, must be http or https")
	}

	host, port := parsedURL.Hostname(), parsedURL.Port()
	return ConnectionString{
		protocol:  protocol,
		host:      host,
		port:      port,
		authToken: parsedURL.Query().Get("authToken"),
	}, nil
}
