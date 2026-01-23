package failure

import (
	"regexp"
	"strconv"
	"strings"
)

// stdlibParser parses basic Go testing failures (t.Error, t.Errorf, t.Fatal, t.Fatalf).
// This is a fallback parser with lower priority than specialized parsers.
type stdlibParser struct{}

var (
	// stdlibLogPattern matches "file.go:123: message" patterns from t.Error/t.Errorf.
	stdlibLogPattern = regexp.MustCompile(`^\s*(\S+\.go):(\d+):\s*(.*)$`)
)

func (p *stdlibParser) Name() string {
	return "stdlib"
}

func (p *stdlibParser) CanParse(output string) bool {
	// look for the file.go:line: pattern that stdlib testing produces
	return stdlibLogPattern.MatchString(output)
}

func (p *stdlibParser) Parse(output string) *StructuredFailure {
	sf := &StructuredFailure{
		FailureType: AssertionFailure,
		RawOutput:   output,
		Assertion: &AssertionInfo{
			Version: AssertionInfoVersion,
			Library: "stdlib",
		},
	}

	lines := strings.Split(output, "\n")
	var messages []string

	for _, line := range lines {
		if matches := stdlibLogPattern.FindStringSubmatch(line); len(matches) >= 4 {
			// first match sets the location
			if sf.Location.IsZero() {
				sf.Location.File = matches[1]
				if lineNum, err := strconv.Atoi(matches[2]); err == nil {
					sf.Location.Line = lineNum
				}
			}
			// collect the message part
			msg := strings.TrimSpace(matches[3])
			if msg != "" {
				messages = append(messages, msg)
			}
		}
	}

	// combine messages
	if len(messages) > 0 {
		sf.Assertion.Message = strings.Join(messages, "\n")
	}

	// try to extract expected/actual from common patterns
	p.extractExpectedActual(sf, output)

	return sf
}

// extractExpectedActual attempts to find expected/actual values in the output.
// Common patterns include "expected X, got Y" and "want X, got Y".
func (p *stdlibParser) extractExpectedActual(sf *StructuredFailure, output string) {
	lower := strings.ToLower(output)

	// pattern: "expected X, got Y" or "expected X but got Y"
	if idx := strings.Index(lower, "expected"); idx >= 0 {
		rest := output[idx:]
		if gotIdx := strings.Index(strings.ToLower(rest), "got"); gotIdx > 0 {
			// extract expected value (between "expected" and "got")
			expected := rest[len("expected"):gotIdx]
			expected = strings.Trim(expected, " ,\t\n")
			expected = strings.TrimSuffix(expected, "but")
			expected = strings.TrimSpace(expected)
			sf.Assertion.Expected = expected

			// extract actual value (after "got")
			actual := rest[gotIdx+len("got"):]
			// clean up common suffixes
			if commaIdx := strings.Index(actual, ","); commaIdx > 0 {
				actual = actual[:commaIdx]
			}
			actual = strings.TrimSpace(actual)
			sf.Assertion.Actual = actual
		}
	}

	// pattern: "want X, got Y"
	if sf.Assertion.Expected == "" {
		if idx := strings.Index(lower, "want"); idx >= 0 {
			rest := output[idx:]
			if gotIdx := strings.Index(strings.ToLower(rest), "got"); gotIdx > 0 {
				// extract expected value
				expected := rest[len("want"):gotIdx]
				expected = strings.Trim(expected, " ,\t\n")
				expected = strings.TrimSpace(expected)
				sf.Assertion.Expected = expected

				// extract actual value
				actual := rest[gotIdx+len("got"):]
				if commaIdx := strings.Index(actual, ","); commaIdx > 0 {
					actual = actual[:commaIdx]
				}
				actual = strings.TrimSpace(actual)
				sf.Assertion.Actual = actual
			}
		}
	}
}
