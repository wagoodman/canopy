package parallel

import (
	"testing"
	"time"
)

func TestNestedParallel1Slow(t *testing.T) {
	t.Parallel()
	time.Sleep(5 * time.Second) // simulate a really slow test

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
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			time.Sleep(tt.amt * time.Millisecond * 175)
		})
	}
}

func TestNestedParallel2(t *testing.T) {
	t.Parallel()
	time.Sleep(500 * time.Millisecond)
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
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			time.Sleep(tt.amt * time.Millisecond * 125)
		})
	}
}
