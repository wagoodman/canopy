package parallel

import (
	"testing"
	"time"
)

func TestOtherParallel1(t *testing.T) {
	t.Parallel()
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
			time.Sleep(tt.amt * time.Millisecond * 350)
			t.Fail()
		})
	}
}

func TestOtherParallel2(t *testing.T) {
	t.Parallel()
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
			time.Sleep(tt.amt * time.Millisecond * 450)
		})
	}
}
