package gotest

import "strings"

const (
	NoTestFiles  Annotation = "no-test-files"   // e.g. "[no test files]"
	NoTestsToRun Annotation = "no-tests-to-run" // e.g. "testing: warning: no tests to run" ... "[no tests to run]"
	Cached       Annotation = "cached"          // e.g. "(cached)"
	BuildFailed  Annotation = "build-failed"    // e.g. "[build failed]"
	SetupFailed  Annotation = "setup-failed"    // e.g. "[setup failed]"
)

// Annotation is a bracketed snippet found in a test event output line, usually at the end.
type Annotation string

func ExtractAnnotations(output string) []Annotation {
	output = strings.TrimSpace(output)
	var annotations []Annotation

	if strings.HasSuffix(output, "[no test files]") {
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
