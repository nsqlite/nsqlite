package config

import (
	"errors"
	"net/url"
)

// ConnectionString represents the connection string for the NSQLite
// database server.
type ConnectionString struct {
	Protocol  string
	Host      string
	Port      string
	AuthToken string
}

// String returns the string representation of the connection string without
// the auth token.
func (c ConnectionString) String() string {
	if c.AuthToken == "" {
		return c.Protocol + "://" + c.Host + ":" + c.Port
	}

	return c.Protocol + "://" + c.Host + ":" + c.Port + "?authToken=****"
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
		Protocol:  protocol,
		Host:      host,
		Port:      port,
		AuthToken: parsedURL.Query().Get("authToken"),
	}, nil
}
