package test_fixtures

import (
	"strings"
	"testing"

	stringsCan "github.com/wagoodman/canopy/internal/test-fixtures/strings"
)

// that's right! a duplicate test from the strings package!
// this helps test uniqueness of test names across the entire test set.

func TestIsPalindrome(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"single word palindrome", "racecar", true},
		{"not palindrome", "hello", false},
		{"single word palindrome", "deified", true},
		{"mix case palindrome", "was it a car or a cat I saw", true},
		{"lower case palindrome", "was it a car or a cat i saw", true},
	}

	for _, tt := range tests {
		if skipOops != nil && *skipOops && strings.Contains(tt.name, "oops") {
			t.Skip()
		}
		t.Logf("this is an exciting log message for %q", tt.name)

		t.Run(tt.name, func(t *testing.T) {
			result := stringsCan.IsPalindrome(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v, but received %v", tt.expected, result)
			}
		})
	}
}
