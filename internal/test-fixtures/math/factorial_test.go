package math

import (
	"flag"
	"testing"
)

var skipOops = flag.Bool("skip-oops", false, "skip tests that have 'oops' in the name")

func TestFactorial(t *testing.T) {
	for _, tt := range []struct {
		name     string
		input    int
		expected int
	}{
		{"test1", 0, 1},
		{"test2", 1, 1},
		{"test3", 5, 120},
		{"test4", -1, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := Factorial(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v, but received %v", tt.expected, result)
			}
		})
	}
}
