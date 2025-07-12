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

func IsLogLine(output string) bool {
	// match regex for a line like this:
	//    palindrome_test.go:51: th
	return logLinePattern.MatchString(output)
}

func HasTestPassMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "--- PASS:")
}

func HasPackageCoverageMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "coverage:")
}

func HasPassedPackageMarking(output string) bool {
	return strings.HasPrefix(output, "ok")
}

func HasUnknownPackageMarking(output string) bool {
	return strings.HasPrefix(output, "?") || strings.HasPrefix(output, "\t")
}

func HasPackagePassMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "PASS")
}

func HasRunMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "=== RUN")
}

func HasConclusionMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "--- ")
}

func HasStateMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "=== ")
}

func HasContinueMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "=== CONT")
}

func HasPauseMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "=== PAUSE")
}

func HasFailedTestMarking(output string) bool {
	return strings.HasPrefix(strings.TrimSpace(output), "--- FAIL:")
}

func HasFailedPackageMarking(output string) bool {
	return strings.HasPrefix(output, "FAIL")
}

func HasPanicMarking(output string) bool {
	return strings.HasPrefix(output, "panic:")
}

func HasTimeMarker(output string) bool {
	return timePattern.MatchString(strings.TrimSpace(output))
}

func IsPanicGoRoutineLine(s string) bool {
	return panicGoroutinePattern.MatchString(s)
}

func IsPanicFuncLine(s string) bool {
	return !strings.HasPrefix(s, "\t") && strings.Contains(s, "(") && strings.Contains(s, ")")
}

func IsPanicFileLine(s string) bool {
	return strings.HasPrefix(s, "\t"+string(os.PathSeparator))
}
