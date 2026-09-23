package gotest

import "strings"

const (
	NoTestFiles  Annotation = "no-test-files"   // e.g. "[no test files]"
	NoTestsToRun Annotation = "no-tests-to-run" // e.g. "testing: warning: no tests to run" ... "[no tests to run]"
	Cached       Annotation = "cached"          // e.g. "(cached)"
	BuildFailed  Annotation = "build-failed"    // e.g. "[build failed]"
	SetupFailed  Annotation = "setup-failed"    // e.g. "[setup failed]"
)

// Annotation represents special conditions extracted from test output, such as cached results
// or build failures. Annotations provide additional context about test execution beyond
// the basic pass/fail status.
type Annotation string

// ExtractAnnotations parses special markers from go test output that indicate
// exceptional conditions. Returns a slice of all annotations found in the output.
// These annotations help distinguish between different types of test failures
// and special states like cached results.
func ExtractAnnotations(output string) []Annotation {
	coverOnly := isCoverOnlyNoTestFiles(output)
	output = strings.TrimSpace(output)
	var annotations []Annotation

	if coverOnly || strings.HasSuffix(output, "[no test files]") {
		annotations = append(annotations, NoTestFiles)
	}

	if strings.HasSuffix(output, "[no tests to run]") {
		annotations = append(annotations, NoTestsToRun)
	}

	// special case: https://github.com/golang/go/blob/c71cbd544e3da139badd4c03612af41b63711705/src/cmd/go/internal/test/test.go#L1224
	if strings.HasSuffix(output, "[build failed]") {
		annotations = append(annotations, BuildFailed)
	}

	// special case: https://github.com/golang/go/blob/c71cbd544e3da139badd4c03612af41b63711705/src/cmd/go/internal/test/test.go#L915
	if strings.HasSuffix(output, "[setup failed]") {
		annotations = append(annotations, SetupFailed)
	}

	if strings.Contains(output, "(cached)") {
		annotations = append(annotations, Cached)
	}

	return annotations
}

// isCoverOnlyNoTestFiles reports whether the output is the line go prints under -cover for a package with no test
// files but some statements: "\tpkg\t\tcoverage: 0.0% of statements" (no status, no elapsed, no "[no test files]").
func isCoverOnlyNoTestFiles(output string) bool {
	fields := strings.Split(strings.TrimRight(output, "\n"), "\t")
	return len(fields) == 4 && fields[0] == "" && fields[2] == "" && strings.HasPrefix(fields[3], "coverage:")
}
