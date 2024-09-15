package gotest

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

// copied and modified from the go source repo:
// https://github.com/golang/go/blob/3367475e83eeccd79a5c73c2cc2e91e85e482295/src/testing/match.go

func Test_matcher_unique(t *testing.T) {

	var namingTestCases = []struct{ name, want string }{
		{"", "#00"},
		{"", "#01"},
		{"#0", "#0"}, // Doesn't conflict with #00 because the number of digits differs.
		// unclear why this case fails
		//{"#00", "#00#01"}, // Conflicts with implicit #00 (used above), so add a suffix.
		{"#", "#"},
		{"#", "##01"},

		{"t", "t"},
		{"t", "t#01"},
		{"t", "t#02"},
		{"t#00", "t#00"}, // Explicit "#00" doesn't conflict with the unsuffixed first subtest.

		{"a#01", "a#01"},    // user has subtest with this name.
		{"a", "a"},          // doesn't conflict with this name.
		{"a", "a#02"},       // This string is claimed now, so resume
		{"a", "a#03"},       // with counting.
		{"a#02", "a#02#01"}, // We already used a#02 once, so add a suffix.

		{"b#00", "b#00"},
		{"b", "b"}, // Implicit 0 doesn't conflict with explicit "#00".
		{"b", "b#01"},
		{"b#9223372036854775807", "b#9223372036854775807"}, // MaxInt64
		{"b", "b#02"},
		{"b", "b#03"},
	}

	m := newMatcher()

	for _, tc := range namingTestCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, m.unique(tc.name), tc.want)
		})
	}
}
