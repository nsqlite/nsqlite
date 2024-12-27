package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseConnectionString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    ConnectionString
		expectError bool
	}{
		{
			name:  "valid connection string with all fields",
			input: "https://localhost:4150?authToken=secret123",
			expected: ConnectionString{
				Protocol:  "https",
				Host:      "localhost",
				Port:      "4150",
				AuthToken: "secret123",
			},
			expectError: false,
		},
		{
			name:  "valid connection string without auth token",
			input: "https://localhost:4150",
			expected: ConnectionString{
				Protocol:  "https",
				Host:      "localhost",
				Port:      "4150",
				AuthToken: "",
			},
			expectError: false,
		},
		{
			name:  "http protocol",
			input: "http://127.0.0.1:8080?authToken=token123",
			expected: ConnectionString{
				Protocol:  "http",
				Host:      "127.0.0.1",
				Port:      "8080",
				AuthToken: "token123",
			},
			expectError: false,
		},
		{
			name:  "connection string with URL encoded characters",
			input: "https://localhost:4150?authToken=secret%20123%26special",
			expected: ConnectionString{
				Protocol:  "https",
				Host:      "localhost",
				Port:      "4150",
				AuthToken: "secret 123&special",
			},
			expectError: false,
		},
		{
			name:  "connection string without port",
			input: "https://localhost?authToken=secret123",
			expected: ConnectionString{
				Protocol:  "https",
				Host:      "localhost",
				Port:      "",
				AuthToken: "secret123",
			},
			expectError: false,
		},
		{
			name:        "empty connection string",
			input:       "",
			expectError: true,
		},
		{
			name:        "invalid URL format",
			input:       "not-a-valid-url",
			expectError: true,
		},
		{
			name:        "missing protocol",
			input:       "://localhost:4150",
			expectError: true,
		},
		{
			name:        "invalid protocol",
			input:       "tcp://localhost:4150",
			expectError: true,
		},
		{
			name:  "IPv6 address",
			input: "https://[::1]:4150?authToken=secret123",
			expected: ConnectionString{
				Protocol:  "https",
				Host:      "::1",
				Port:      "4150",
				AuthToken: "secret123",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseConnectionString(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
