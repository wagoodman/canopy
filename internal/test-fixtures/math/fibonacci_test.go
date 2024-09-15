package math

import (
	"reflect"
	"testing"
)

func TestFibonacci(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected []int
	}{
		{"test1", 1, []int{0}},
		{"test2", 2, []int{0, 1}},
		{"test3", 5, []int{0, 1, 1, 2, 3}},
		{"test4", 10, []int{0, 1, 1, 2, 3, 5, 8, 13, 21, 34}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Fibonacci(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, but received %v", tt.expected, result)
			}
		})
	}
}
