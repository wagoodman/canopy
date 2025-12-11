package internal

import (
	"fmt"
	"regexp"
)

// MatchesAny returns true if the statement matches any of the provided regex patterns.
// If no patterns are provided, it returns true (meaning all statements match by default).
func MatchesAny(statement string, runPatterns []*regexp.Regexp) bool {
	if len(runPatterns) == 0 {
		return true // no run patterns means all statements match
	}
	for _, run := range runPatterns {
		if run.MatchString(statement) {
			return true
		}
	}
	return false
}

// MakeRegexes compiles a slice of regex pattern strings into compiled regexp objects.
// Empty strings are skipped. Returns an error if any pattern fails to compile.
func MakeRegexes(runStatements ...string) ([]*regexp.Regexp, error) {
	var regexes []*regexp.Regexp
	for _, run := range runStatements {
		if run == "" {
			continue
		}
		re, err := regexp.Compile(run)
		if err != nil {
			return nil, fmt.Errorf("failed to compile regex %q: %w", run, err)
		}
		regexes = append(regexes, re)
	}
	return regexes, nil
}
