package output

import (
	"os"
	"regexp"
	"strings"
)

var (
	logLinePattern        = regexp.MustCompile(`^\s*\S+.go:\d+:`)
	timePattern           = regexp.MustCompile(`^\d+\.?\d*\S+$`)
	panicGoroutinePattern = regexp.MustCompile(`^goroutine \d+`)
)

// Indicator is a function that examines test output and returns true if it matches a specific pattern.
type Indicator func(string) bool

// HasAny returns an Indicator that matches if any of the provided indicators match.
// This is useful for checking multiple conditions with OR logic.
func HasAny(indicators ...Indicator) Indicator {
	return func(output string) bool {
		for _, indicator := range indicators {
			if indicator(output) {
				return true
			}
		}
		return false
	}
}

// HasAll returns an Indicator that matches only if all provided indicators match.
// This is useful for checking multiple conditions with AND logic.
func HasAll(indicators ...Indicator) Indicator {
	return func(output string) bool {
		for _, indicator := range indicators {
			if !indicator(output) {
				return false
			}
		}
		return true
	}
}

// IsLogLine returns true if the output matches the pattern for a log line.
// Log lines typically look like "palindrome_test.go:51: message".
func IsLogLine(output string) bool {
	// match regex for a line like this:
	//    palindrome_test.go:51: th
	return logLinePattern.MatchString(output)
}

// HasUnknownPackageMarking returns true if the output indicates an unknown or untestable package.
func HasUnknownPackageMarking(output string) bool {
	return strings.HasPrefix(output, "?") || strings.HasPrefix(output, "\t")
}

// Execution state markings...
// These are used to indicate when there has been a lifecycle event in test execution (run/pause/continue).

// HasStateMarking returns true if the output contains a test state transition marker.
func HasStateMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "=== ")
}

// HasRunMarking returns true if the output indicates a test is starting.
func HasRunMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "=== RUN")
}

// HasContinueMarking returns true if the output indicates a test continuation after pause.
func HasContinueMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "=== CONT")
}

// HasPauseMarking returns true if the output indicates a test pause (for t.Parallel).
func HasPauseMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "=== PAUSE")
}

// Conclusion markings...
// These are used to indicate the result of a test (pass/fail/skip).

// HasConclusionMarking returns true if the output contains a test conclusion marker.
func HasConclusionMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "--- ")
}

// HasFailedTestMarking returns true if the output contains a test failure marker.
func HasFailedTestMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "--- FAIL:")
}

// HasTestPassMarking returns true if the output contains a test pass marker.
func HasTestPassMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "--- PASS:")
}

// Package level conclusion markings...
// These are used to indicate the result of a package's tests (pass/fail).

// HasPackagePassMarking returns true if the output contains a package-level PASS marker.
func HasPackagePassMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "PASS")
}

// HasFailedPackageMarking returns true if the output indicates a package failure.
func HasFailedPackageMarking(output string) bool {
	return strings.HasPrefix(output, "FAIL")
}

// HasFailedPackageTrailer returns true if the output is a package failure trailer line.
func HasFailedPackageTrailer(output string) bool {
	return strings.HasPrefix(output, "FAIL\n")
}

// HasPackageCoverageMarking returns true if the output contains package coverage information.
func HasPackageCoverageMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "coverage:")
}

// HasPackageOKMarking returns true if the output indicates a package passed.
func HasPackageOKMarking(output string) bool {
	return strings.HasPrefix(output, "ok")
}

// Other markings...

// HasPanicMarking returns true if the output indicates a panic occurred.
func HasPanicMarking(output string) bool {
	return strings.HasPrefix(output, "panic:")
}

// HasTimeMarker returns true if the output contains a time duration marker.
func HasTimeMarker(output string) bool {
	return timePattern.MatchString(strings.TrimSpace(output))
}

// IsPanicGoRoutineLine returns true if the output is a panic goroutine identifier line.
func IsPanicGoRoutineLine(s string) bool {
	return panicGoroutinePattern.MatchString(s)
}

// IsPanicFuncLine returns true if the output is a panic function call line.
func IsPanicFuncLine(s string) bool {
	return !strings.HasPrefix(s, "\t") && strings.Contains(s, "(") && strings.Contains(s, ")")
}

// IsPanicFileLine returns true if the output is a panic file location line.
func IsPanicFileLine(s string) bool {
	return strings.HasPrefix(s, "\t"+string(os.PathSeparator))
}

// IsWhitespace returns true if the string contains only whitespace characters.
func IsWhitespace(s string) bool {
	return len(s) > 0 && strings.TrimSpace(s) == ""
}
