package failure

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestTestifyParser_CanParse(t *testing.T) {
	parser := &testifyParser{}

	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{
			name: "typical testify output",
			input: `    handler_test.go:42:
        	Error Trace:	handler_test.go:42
        	Error:      	Not equal:
        	            	expected: "success"
        	            	actual  : "unauthorized"
        	Test:       	TestUserLogin`,
			expect: true,
		},
		{
			name:   "no error trace",
			input:  "some error message without Error Trace",
			expect: false,
		},
		{
			name:   "panic output",
			input:  "panic: runtime error: nil pointer",
			expect: false,
		},
		{
			name: "error trace present",
			input: `Error Trace: file.go:10
Error: something failed`,
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.CanParse(tt.input)
			require.Equal(t, tt.expect, got)
		})
	}
}

func TestTestifyParser_Parse(t *testing.T) {
	parser := &testifyParser{}

	tests := []struct {
		name   string
		input  string
		expect *StructuredFailure
	}{
		{
			name: "equal assertion failure",
			input: `    handler_test.go:42:
        	Error Trace:	handler_test.go:42
        	Error:      	Not equal:
        	            	expected: "success"
        	            	actual  : "unauthorized"
        	Test:       	TestUserLogin`,
			expect: &StructuredFailure{
				FailureType: AssertionFailure,
				Assertion: &AssertionInfo{
					Library:  "testify",
					Function: "Equal",
					Expected: `"success"`,
					Actual:   `"unauthorized"`,
				},
				Location: SourceLocation{
					File: "handler_test.go",
					Line: 42,
				},
			},
		},
		{
			name: "nil assertion failure",
			input: `Error Trace:	user_test.go:55
Error:      	Expected nil, but got: &{ID:1 Name:"test"}
Test:       	TestCreateUser`,
			expect: &StructuredFailure{
				FailureType: AssertionFailure,
				Assertion: &AssertionInfo{
					Library:  "testify",
					Function: "Nil",
				},
				Location: SourceLocation{
					File: "user_test.go",
					Line: 55,
				},
			},
		},
		{
			name: "with custom message",
			input: `Error Trace:	api_test.go:100
Error:      	Not equal:
            	expected: 200
            	actual  : 500
Messages:   	status code mismatch
Test:       	TestAPIEndpoint`,
			expect: &StructuredFailure{
				FailureType: AssertionFailure,
				Assertion: &AssertionInfo{
					Library:  "testify",
					Function: "Equal",
					Expected: "200",
					Actual:   "500",
					Message:  "status code mismatch",
				},
				Location: SourceLocation{
					File: "api_test.go",
					Line: 100,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.Parse(tt.input)
			require.NotNil(t, got)

			// check type
			require.Equal(t, tt.expect.FailureType, got.FailureType)

			// check location
			require.Equal(t, tt.expect.Location.File, got.Location.File)
			require.Equal(t, tt.expect.Location.Line, got.Location.Line)

			// check assertion info
			if tt.expect.Assertion != nil {
				require.NotNil(t, got.Assertion)
				require.Equal(t, tt.expect.Assertion.Library, got.Assertion.Library)
				require.Equal(t, tt.expect.Assertion.Function, got.Assertion.Function)
				if tt.expect.Assertion.Expected != "" {
					require.Equal(t, tt.expect.Assertion.Expected, got.Assertion.Expected)
				}
				if tt.expect.Assertion.Actual != "" {
					require.Equal(t, tt.expect.Assertion.Actual, got.Assertion.Actual)
				}
				if tt.expect.Assertion.Message != "" {
					require.Equal(t, tt.expect.Assertion.Message, got.Assertion.Message)
				}
			}

			// verify raw output is preserved
			require.Equal(t, tt.input, got.RawOutput)
		})
	}
}

func TestTestifyParser_ExtractFunction(t *testing.T) {
	parser := &testifyParser{}

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "not equal message",
			input:  "Not equal: expected vs actual",
			expect: "Equal",
		},
		{
			name:   "expected nil",
			input:  "Expected nil, but got something",
			expect: "Nil",
		},
		{
			name:   "should be true",
			input:  "Should be true but was false",
			expect: "True",
		},
		{
			name:   "should contain",
			input:  "Should contain substring",
			expect: "Contains",
		},
		{
			name:   "no match",
			input:  "some random error",
			expect: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.extractFunction(tt.input)
			require.Equal(t, tt.expect, got)
		})
	}
}

func TestTestifyParser_ComplexOutput(t *testing.T) {
	parser := &testifyParser{}

	// test with a more complex real-world output
	input := `    --- FAIL: TestGetUser (0.00s)
        user_test.go:123:
        	Error Trace:	/home/user/project/user_test.go:123
        	Error:      	Not equal:
        	            	expected: map[string]interface {}{"email":"test@example.com", "id":"123", "name":"Test User"}
        	            	actual  : map[string]interface {}{"email":"wrong@example.com", "id":"123", "name":"Test User"}

        	            	Diff:
        	            	--- Expected
        	            	+++ Actual
        	            	@@ -1,3 +1,3 @@
        	            	 (map[string]interface {}) (len=3) {
        	            	- (string) (len=5) "email": (string) (len=16) "test@example.com",
        	            	+ (string) (len=5) "email": (string) (len=17) "wrong@example.com",
        	            	  (string) (len=2) "id": (string) (len=3) "123",
        	Test:       	TestGetUser`

	got := parser.Parse(input)

	require.NotNil(t, got)
	require.Equal(t, AssertionFailure, got.FailureType)
	require.NotNil(t, got.Assertion)
	require.Equal(t, "testify", got.Assertion.Library)
	require.Equal(t, "Equal", got.Assertion.Function)
	require.Equal(t, "/home/user/project/user_test.go", got.Location.File)
	require.Equal(t, 123, got.Location.Line)

	// expected/actual should contain the map representations
	require.Contains(t, got.Assertion.Expected, "test@example.com")
	require.Contains(t, got.Assertion.Actual, "wrong@example.com")

	_ = cmp.Diff(1, 2) // ensure cmp is available for reference
}
