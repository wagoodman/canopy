package failure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStdlibParser_CanParse(t *testing.T) {
	parser := &stdlibParser{}

	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{
			name:   "file line message format",
			input:  "    test_file.go:42: expected 1, got 2",
			expect: true,
		},
		{
			name:   "no indent",
			input:  "test.go:10: error",
			expect: true,
		},
		{
			name:   "testify output",
			input:  "Error Trace: file.go:10",
			expect: false,
		},
		{
			name:   "panic output",
			input:  "panic: error",
			expect: false,
		},
		{
			name:   "no file pattern",
			input:  "just a plain error message",
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.CanParse(tt.input)
			require.Equal(t, tt.expect, got)
		})
	}
}

func TestStdlibParser_Parse(t *testing.T) {
	parser := &stdlibParser{}

	tests := []struct {
		name   string
		input  string
		expect *StructuredFailure
	}{
		{
			name:  "simple error",
			input: "    test_file.go:42: something failed",
			expect: &StructuredFailure{
				FailureType: AssertionFailure,
				Assertion: &AssertionInfo{
					Library: "stdlib",
					Message: "something failed",
				},
				Location: SourceLocation{
					File: "test_file.go",
					Line: 42,
				},
			},
		},
		{
			name:  "expected got pattern",
			input: "    math_test.go:100: expected 10, got 5",
			expect: &StructuredFailure{
				FailureType: AssertionFailure,
				Assertion: &AssertionInfo{
					Library:  "stdlib",
					Expected: "10",
					Actual:   "5",
					Message:  "expected 10, got 5",
				},
				Location: SourceLocation{
					File: "math_test.go",
					Line: 100,
				},
			},
		},
		{
			name:  "want got pattern",
			input: `    handler_test.go:55: want "success", got "error"`,
			expect: &StructuredFailure{
				FailureType: AssertionFailure,
				Assertion: &AssertionInfo{
					Library:  "stdlib",
					Expected: `"success"`,
					Actual:   `"error"`,
					Message:  `want "success", got "error"`,
				},
				Location: SourceLocation{
					File: "handler_test.go",
					Line: 55,
				},
			},
		},
		{
			name: "multiline error",
			input: `    api_test.go:10: first error
    api_test.go:11: second error`,
			expect: &StructuredFailure{
				FailureType: AssertionFailure,
				Assertion: &AssertionInfo{
					Library: "stdlib",
					Message: "first error\nsecond error",
				},
				Location: SourceLocation{
					File: "api_test.go",
					Line: 10,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.Parse(tt.input)
			require.NotNil(t, got)

			require.Equal(t, tt.expect.FailureType, got.FailureType)
			require.NotNil(t, got.Assertion)
			require.Equal(t, tt.expect.Assertion.Library, got.Assertion.Library)

			if tt.expect.Assertion.Message != "" {
				require.Equal(t, tt.expect.Assertion.Message, got.Assertion.Message)
			}
			if tt.expect.Assertion.Expected != "" {
				require.Equal(t, tt.expect.Assertion.Expected, got.Assertion.Expected)
			}
			if tt.expect.Assertion.Actual != "" {
				require.Equal(t, tt.expect.Assertion.Actual, got.Assertion.Actual)
			}

			require.Equal(t, tt.expect.Location.File, got.Location.File)
			require.Equal(t, tt.expect.Location.Line, got.Location.Line)

			// verify raw output preserved
			require.Equal(t, tt.input, got.RawOutput)
		})
	}
}

func TestStdlibParser_ExtractExpectedActual(t *testing.T) {
	parser := &stdlibParser{}

	tests := []struct {
		name           string
		input          string
		expectExpected string
		expectActual   string
	}{
		{
			name:           "expected comma got",
			input:          "expected 10, got 5",
			expectExpected: "10",
			expectActual:   "5",
		},
		{
			name:           "expected but got",
			input:          "expected true but got false",
			expectExpected: "true",
			expectActual:   "false",
		},
		{
			name:           "want comma got",
			input:          `want "foo", got "bar"`,
			expectExpected: `"foo"`,
			expectActual:   `"bar"`,
		},
		{
			name:           "no pattern",
			input:          "something went wrong",
			expectExpected: "",
			expectActual:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &StructuredFailure{
				Assertion: &AssertionInfo{},
			}
			parser.extractExpectedActual(sf, tt.input)
			require.Equal(t, tt.expectExpected, sf.Assertion.Expected)
			require.Equal(t, tt.expectActual, sf.Assertion.Actual)
		})
	}
}
