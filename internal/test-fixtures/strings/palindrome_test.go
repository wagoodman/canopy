package strings

import (
	"flag"
	"strings"
	"testing"
	"time"
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
		{"mix case palindrome (oops)", "was it a car or a cat I SAW", false},
		{"lower case palindrome", "was it a car or a cat i saw", true},
	}

	for _, tt := range tests {
		if skipOops != nil && *skipOops && strings.Contains(tt.name, "oops") {
			t.Skip()
		}
		t.Logf("this is an exciting log message for %q", tt.name)

		time.Sleep(200 * time.Millisecond)

		t.Run(tt.name, func(t *testing.T) {
			t.Log("this is a nested log message for:", tt.name)
			t.Run("subtest", func(t *testing.T) {
				t.Log("this is a nested nested log message for:", tt.name)
				if strings.Contains(tt.name, "oops") {
					t.Errorf("this is a nested nested error (%s)", tt.name)
				}
			})
			//if strings.Contains(tt.name, "oops") {
			//	panic("erf")
			//}
			result := IsPalindrome(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v, but received %v", tt.expected, result)
			}
		})

		t.Logf("this is an exciting log message for %q ... happening at the bottom of the loop", tt.name)

	}
}
