package httputil

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadReqBodyBytes(t *testing.T) {
	tests := []struct {
		name    string
		req     *http.Request
		want    []byte
		wantErr string
	}{
		{
			name:    "nil request",
			req:     nil,
			want:    nil,
			wantErr: "request cannot be nil",
		},
		{
			name:    "nil body",
			req:     &http.Request{Body: nil},
			want:    nil,
			wantErr: "",
		},
		{
			name: "empty body",
			req: &http.Request{
				Body: io.NopCloser(bytes.NewReader([]byte{})),
			},
			want:    []byte{},
			wantErr: "",
		},
		{
			name: "valid body",
			req: &http.Request{
				Body: io.NopCloser(bytes.NewReader([]byte("hello world"))),
			},
			want:    []byte("hello world"),
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadReqBodyBytes(tt.req)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
