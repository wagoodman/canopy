package failure

import (
	"regexp"
	"strconv"
	"strings"
)

// testifyParser parses testify assertion failures.
// Testify output follows a consistent format with Error Trace, Error, and Test markers.
type testifyParser struct{}

var (
	// errorTracePattern matches "Error Trace:  file.go:123" lines.
	errorTracePattern = regexp.MustCompile(`Error Trace:\s*(.+?):(\d+)`)
	// testifyExpectedPattern matches "expected: value" lines.
	testifyExpectedPattern = regexp.MustCompile(`(?m)^\s*expected:\s*(.+)$`)
	// testifyActualPattern matches "actual  : value" lines (note extra spaces for alignment).
	testifyActualPattern = regexp.MustCompile(`(?m)^\s*actual\s*:\s*(.+)$`)
	// testifyMessagePattern matches "Message: value" lines.
	testifyMessagePattern = regexp.MustCompile(`(?m)^\s*Messages?:\s*(.+)$`)
	// testifyFuncPattern matches common testify assertion function names in the trace.
	testifyFuncPattern = regexp.MustCompile(`\.(Equal|NotEqual|Contains|NotContains|Nil|NotNil|True|False|Error|NoError|Empty|NotEmpty|Len|Greater|Less|GreaterOrEqual|LessOrEqual|ElementsMatch|Subset|JSONEq|Regexp|NotRegexp|IsType|Implements|PanicsWithValue|Panics|ErrorIs|ErrorAs|ErrorContains|Eventually|Never|Condition|Same|NotSame|InDelta|InEpsilon|EqualValues|EqualExportedValues|WithinDuration|Zero|NotZero|FileExists|NoFileExists|DirExists|NoDirExists)\(`)
)

// testify output markers
const (
	errorTraceMarker = "Error Trace:"
	errorMarker      = "Error:"
	testMarker       = "Test:"
)

func (p *testifyParser) Name() string {
	return "testify"
}

func (p *testifyParser) CanParse(output string) bool {
	// testify failures have the Error Trace marker
	return strings.Contains(output, errorTraceMarker)
}

func (p *testifyParser) Parse(output string) *StructuredFailure {
	sf := &StructuredFailure{
		FailureType: AssertionFailure,
		RawOutput:   output,
		Assertion: &AssertionInfo{
			Version: AssertionInfoVersion,
			Library: "testify",
		},
	}

	// extract source location from Error Trace
	if matches := errorTracePattern.FindStringSubmatch(output); len(matches) >= 3 {
		sf.Location.File = matches[1]
		if line, err := strconv.Atoi(matches[2]); err == nil {
			sf.Location.Line = line
		}
	}

	// extract expected value
	if matches := testifyExpectedPattern.FindStringSubmatch(output); len(matches) >= 2 {
		sf.Assertion.Expected = strings.TrimSpace(matches[1])
		// handle multi-line expected values
		sf.Assertion.Expected = p.extractMultilineValue(output, "expected:", sf.Assertion.Expected)
	}

	// extract actual value
	if matches := testifyActualPattern.FindStringSubmatch(output); len(matches) >= 2 {
		sf.Assertion.Actual = strings.TrimSpace(matches[1])
		// handle multi-line actual values - note: "actual" marker can have varying whitespace
		sf.Assertion.Actual = p.extractMultilineValue(output, "actual  :", sf.Assertion.Actual)
	}

	// extract message if present
	if matches := testifyMessagePattern.FindStringSubmatch(output); len(matches) >= 2 {
		sf.Assertion.Message = strings.TrimSpace(matches[1])
	}

	// try to extract the assertion function name
	sf.Assertion.Function = p.extractFunction(output)

	return sf
}

// extractMultilineValue attempts to extract multi-line values that span multiple lines.
// This handles cases where expected/actual values contain newlines.
func (p *testifyParser) extractMultilineValue(output, marker, initial string) string {
	lines := strings.Split(output, "\n")
	var collecting bool
	var result []string

lineLoop:
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(strings.ToLower(trimmed), marker) {
			collecting = true
			// extract the value after the marker on this line
			idx := strings.Index(strings.ToLower(trimmed), marker)
			if idx >= 0 {
				val := strings.TrimSpace(trimmed[idx+len(marker):])
				if val != "" {
					result = append(result, val)
				}
			}
			continue
		}

		if collecting {
			// stop collecting when we hit another known marker
			if strings.HasPrefix(trimmed, "actual") ||
				strings.HasPrefix(trimmed, "expected:") ||
				strings.HasPrefix(trimmed, "Message") ||
				strings.HasPrefix(trimmed, "Test:") ||
				strings.HasPrefix(trimmed, "Error:") ||
				strings.HasPrefix(trimmed, "Error Trace:") {
				break
			}

			// continuation lines are typically indented
			switch {
			case strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "  "):
				if trimmed != "" {
					result = append(result, trimmed)
				}
			case trimmed == "":
				// empty line might end the value
				continue
			default:
				// non-indented non-marker line ends collection
				break lineLoop
			}
		}
	}

	if len(result) > 0 {
		return strings.Join(result, "\n")
	}
	return initial
}

// extractFunction attempts to identify the testify assertion function from the output.
func (p *testifyParser) extractFunction(output string) string {
	if matches := testifyFuncPattern.FindStringSubmatch(output); len(matches) >= 2 {
		return matches[1]
	}

	// try to infer from the error message pattern
	lowerOutput := strings.ToLower(output)

	switch {
	case strings.Contains(lowerOutput, "not equal"):
		return "Equal"
	case strings.Contains(lowerOutput, "expected nil"):
		return "Nil"
	case strings.Contains(lowerOutput, "should be nil"):
		return "Nil"
	case strings.Contains(lowerOutput, "should not be nil"):
		return "NotNil"
	case strings.Contains(lowerOutput, "expected not nil"):
		return "NotNil"
	case strings.Contains(lowerOutput, "should be true"):
		return "True"
	case strings.Contains(lowerOutput, "should be false"):
		return "False"
	case strings.Contains(lowerOutput, "should contain"):
		return "Contains"
	case strings.Contains(lowerOutput, "should not contain"):
		return "NotContains"
	case strings.Contains(lowerOutput, "error is expected"):
		return "Error"
	case strings.Contains(lowerOutput, "no error is expected"):
		return "NoError"
	case strings.Contains(lowerOutput, "should be empty"):
		return "Empty"
	case strings.Contains(lowerOutput, "should not be empty"):
		return "NotEmpty"
	}

	return ""
}
