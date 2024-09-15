package case_1

import (
	"flag"
	"strings"
	"testing"
)

var skipOops = flag.Bool("skip-oops", false, "skip tests that have 'oops' in the name")

func TestIsPalindrome(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"single word palindrome", "racecar", true},
		{"not palindrome", "hello", false},
		{"single word palindrome", "deified", true},
		{"mix case palindrome (oops)", "was it a car or a cat I saw", false},
		{"lower case palindrome", "was it a car or a cat i saw", true},
	}

	for _, tt := range tests {
		if skipOops != nil && *skipOops && strings.Contains(tt.name, "oops") {
			t.Skip()
		}
		t.Run(tt.name, func(t *testing.T) {
			result := IsPalindrome(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v, but received %v", tt.expected, result)
			}
		})
	}
}

// stub, doesn't really matter... just don't like red squiggles
func IsPalindrome(_ string) bool {
	return false
}
