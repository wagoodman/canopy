package case_3

import "testing"

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

// stub, doesn't really matter... just don't like red squiggles
func Factorial(_ int) int {
	return -1
}
