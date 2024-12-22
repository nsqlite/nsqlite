package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_validateListenAddr(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{
			name:    "valid ip address",
			addr:    "127.0.0.1",
			wantErr: false,
		},
		{
			name:    "valid ip address with CIDR",
			addr:    "192.168.1.1/24",
			wantErr: false,
		},
		{
			name:    "valid ip address zeros",
			addr:    "0.0.0.0",
			wantErr: false,
		},
		{
			name:    "invalid string",
			addr:    "invalid",
			wantErr: true,
		},
		{
			name:    "empty string",
			addr:    "",
			wantErr: true,
		},
		{
			name:    "invalid format with dots",
			addr:    "192.168.1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateListenAddr(tt.addr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_validateListenPort(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		wantErr bool
	}{
		{
			name:    "valid port number",
			port:    "8080",
			wantErr: false,
		},
		{
			name:    "valid minimum port",
			port:    "1",
			wantErr: false,
		},
		{
			name:    "valid maximum port",
			port:    "65535",
			wantErr: false,
		},
		{
			name:    "invalid port - letters",
			port:    "abc",
			wantErr: true,
		},
		{
			name:    "invalid port - empty",
			port:    "",
			wantErr: true,
		},
		{
			name:    "invalid port - special chars",
			port:    "8080!",
			wantErr: true,
		},
		{
			name:    "invalid port - decimal",
			port:    "8080.1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateListenPort(tt.port)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_validateAuthAlgorithm(t *testing.T) {
	tests := []struct {
		name      string
		algorithm string
		wantErr   bool
	}{
		{
			name:      "valid - plaintext",
			algorithm: "plaintext",
			wantErr:   false,
		},
		{
			name:      "valid - sha256",
			algorithm: "sha256",
			wantErr:   false,
		},
		{
			name:      "valid - bcrypt",
			algorithm: "bcrypt",
			wantErr:   false,
		},
		{
			name:      "invalid - empty string",
			algorithm: "",
			wantErr:   true,
		},
		{
			name:      "invalid - unknown algorithm",
			algorithm: "md5",
			wantErr:   true,
		},
		{
			name:      "invalid - case sensitive",
			algorithm: "PLAINTEXT",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAuthTokenAlgorithm(tt.algorithm)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.algorithm != "" {
					assert.Contains(t, err.Error(), "valid values are")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
