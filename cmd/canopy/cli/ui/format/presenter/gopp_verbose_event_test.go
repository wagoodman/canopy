package presenter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitWhitespace(t *testing.T) {
	tests := []struct {
		input           string
		expectedPrefix  string
		expectedContent string
	}{
		{"   leading spaces", "   ", "leading spaces"},
		{"\t\ttabbed content", "\t\t", "tabbed content"},
		{"\n\rnewlines", "\n\r", "newlines"},
		{"noWhitespace", "", "noWhitespace"},
		{"  mixed whitespace\n\tand content", "  ", "mixed whitespace\n\tand content"},
		{"", "", ""},
		{"   ", "   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			prefix, content := splitWhitespace(tt.input)
			assert.Equal(t, tt.expectedPrefix, prefix, "Prefix should match expected")
			assert.Equal(t, tt.expectedContent, content, "Content should match expected")
		})
	}
}
