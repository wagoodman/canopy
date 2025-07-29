package internal

import (
	"fmt"
	"regexp"
)

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
