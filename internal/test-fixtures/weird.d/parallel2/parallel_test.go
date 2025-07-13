package parallel

import (
	"testing"
	"time"
)

func TestParallel1(t *testing.T) {
	t.Parallel()
	t.Log("1: Starting parallel tests")
	testCases := []struct {
		name string
		amt  time.Duration
	}{
		{"1 sleep a little", 1},
		{"1 sleep just a little", 1},
		{"1 sleep some more", 2},
		{"1 sleep some more now", 2},
		{"1 sleep even more now!", 3},
		{"1 sleep a lot more", 4},
	}

	for _, tt := range testCases {
		t.Log("1: Starting parallel test case", tt.name)
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Logf("1: Running test %s with sleep duration %v", tt.name, tt.amt)
			time.Sleep(tt.amt * time.Millisecond * 150)
		})
	}
}

func TestParallel2(t *testing.T) {
	t.Parallel()
	t.Log("2: Starting parallel tests")
	testCases := []struct {
		name string
		amt  time.Duration
	}{
		{"2 sleep a little", 1},
		{"2 sleep just a little", 1},
		{"2 sleep some more", 2},
		{"2 sleep some more now", 2},
		{"2 sleep even more now!", 3},
		{"2 sleep a lot more", 4},
	}

	for _, tt := range testCases {
		t.Log("2: Starting parallel test case", tt.name)
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Logf("2: Running test %s with sleep duration %v", tt.name, tt.amt)
			time.Sleep(tt.amt * time.Millisecond * 250)
		})
	}
}
