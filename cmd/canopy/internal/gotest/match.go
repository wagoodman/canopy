package gotest

import (
	"fmt"
	"strconv"
	"strings"
)

// copied from the go source repo:
// https://github.com/golang/go/blob/3367475e83eeccd79a5c73c2cc2e91e85e482295/src/testing/match.go

// matcher sanitizes, uniques, and filters names of subtests and subbenchmarks.
type matcher struct {

	// subNames is used to deduplicate subtest names.
	// Each key is the subtest name joined to the deduplicated name of the parent test.
	// Each value is the count of the number of occurrences of the given subtest name
	// already seen.
	subNames map[string]int64
}

func newMatcher() *matcher {
	return &matcher{
		subNames: make(map[string]int64),
	}
}

// unique creates a unique name for the given parent and subname by affixing it
// with one or more counts, if necessary.
func (m *matcher) unique(subname string) string {
	base := subname

	for {
		n := m.subNames[base]
		if n < 0 {
			panic("subtest count overflow")
		}
		m.subNames[base] = n + 1

		if n == 0 && subname != "" {
			prefix, nn := parseSubtestNumber(base)
			if len(prefix) < len(base) && nn < m.subNames[prefix] {
				// This test is explicitly named like "parent/subname#NN",
				// and #NN was already used for the NNth occurrence of "parent/subname".
				// Loop to add a disambiguating suffix.
				continue
			}
			return base
		}

		name := fmt.Sprintf("%s#%02d", base, n)
		if m.subNames[name] != 0 {
			// This is the nth occurrence of base, but the name "parent/subname#NN"
			// collides with the first occurrence of a subtest *explicitly* named
			// "parent/subname#NN". Try the next number.
			continue
		}

		return name
	}
}

// parseSubtestNumber splits a subtest name into a "#%02d"-formatted int32
// suffix (if present), and a prefix preceding that suffix (always).
func parseSubtestNumber(s string) (prefix string, nn int64) {
	i := strings.LastIndex(s, "#")
	if i < 0 {
		return s, 0
	}

	prefix, suffix := s[:i], s[i+1:]
	if len(suffix) < 2 || (len(suffix) > 2 && suffix[0] == '0') {
		// Even if suffix is numeric, it is not a possible output of a "%02" format
		// string: it has either too few digits or too many leading zeroes.
		return s, 0
	}
	if suffix == "00" {
		if !strings.HasSuffix(prefix, "/") {
			// We only use "#00" as a suffix for subtests named with the empty
			// string — it isn't a valid suffix if the subtest name is non-empty.
			return s, 0
		}
	}

	n, err := strconv.ParseInt(suffix, 10, 32)
	if err != nil || n < 0 {
		return s, 0
	}
	return prefix, n
}
