package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContentType(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		allowTypes  []contentType
		expectMatch bool
	}{
		{
			name:        "match single plain text",
			target:      "text/plain",
			allowTypes:  []contentType{ContentTypePlainText},
			expectMatch: true,
		},
		{
			name:        "match one of multiple types",
			target:      "application/json",
			allowTypes:  []contentType{ContentTypePlainText, ContentTypeJSON, ContentTypeXML},
			expectMatch: true,
		},
		{
			name:        "no match single type",
			target:      "image/jpeg",
			allowTypes:  []contentType{ContentTypePlainText},
			expectMatch: false,
		},
		{
			name:        "no match multiple types",
			target:      "image/png",
			allowTypes:  []contentType{ContentTypePlainText, ContentTypeJSON, ContentTypeXML},
			expectMatch: false,
		},
		{
			name:        "empty target string",
			target:      "",
			allowTypes:  []contentType{ContentTypePlainText},
			expectMatch: false,
		},
		{
			name:        "no allowed types",
			target:      "text/plain",
			allowTypes:  []contentType{},
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContentType(tt.target, tt.allowTypes...)
			assert.Equal(t, tt.expectMatch, result)
		})
	}
}
