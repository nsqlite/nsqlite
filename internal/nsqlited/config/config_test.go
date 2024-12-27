package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
			algorithm: "argon2",
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

func Test_validateTransactionTimeout(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		wantErr  bool
	}{
		{
			name:     "valid - 1 second",
			duration: time.Second,
			wantErr:  false,
		},
		{
			name:     "valid - 1 minute",
			duration: time.Minute,
			wantErr:  false,
		},
		{
			name:     "invalid - negative",
			duration: -time.Second,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransactionTimeout(tt.duration)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
