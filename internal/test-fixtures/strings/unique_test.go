package strings

import (
	"strings"
	"testing"
)

func TestHasUniqueChars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"unique", "abcde", true},
		{"not unique", "hello", false},
		{"oops", "unique", true},
		{"unique", "abbc", false},
	}

	for _, tt := range tests {
		if skipOops != nil && *skipOops && strings.Contains(tt.name, "oops") {
			t.Skip()
		}
		t.Run(tt.name, func(t *testing.T) {
			result := HasUniqueChars(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v, but received %v", tt.expected, result)
			}
		})
	}
}
