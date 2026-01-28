package failure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	require.NotNil(t, registry)

	parsers := registry.Parsers()
	require.Len(t, parsers, 4)

	// verify parser order
	require.Equal(t, "testify", parsers[0].Name())
	require.Equal(t, "panic", parsers[1].Name())
	require.Equal(t, "diff", parsers[2].Name())
	require.Equal(t, "stdlib", parsers[3].Name())
}

func TestRegistry_Parse(t *testing.T) {
	registry := NewRegistry()

	tests := []struct {
		name         string
		input        string
		expectType   Type
		expectParser string
	}{
		{
			name: "testify assertion",
			input: `Error Trace:	test.go:42
Error:      	Not equal:
            	expected: "a"
            	actual  : "b"`,
			expectType:   AssertionFailure,
			expectParser: "testify",
		},
		{
			name: "panic",
			input: `panic: runtime error: nil pointer
goroutine 1 [running]:
main.test()
	/main.go:10 +0x50`,
			expectType:   PanicFailure,
			expectParser: "panic",
		},
		{
			name: "diff",
			input: `(-want +got):
- old
+ new`,
			expectType:   DiffFailure,
			expectParser: "diff",
		},
		{
			name:         "stdlib fallback",
			input:        "    test.go:42: error message",
			expectType:   AssertionFailure,
			expectParser: "stdlib",
		},
		{
			name:         "unknown",
			input:        "some random output without patterns",
			expectType:   UnknownFailure,
			expectParser: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := registry.Parse(tt.input)
			require.NotNil(t, got)
			require.Equal(t, tt.expectType, got.FailureType)
			require.NotEmpty(t, got.Fingerprint)
			require.Equal(t, tt.input, got.RawOutput)

			// verify the correct parser was used based on the result
			if tt.expectParser != "" {
				switch tt.expectParser {
				case "testify":
					require.NotNil(t, got.Assertion)
					require.Equal(t, "testify", got.Assertion.Library)
				case "panic":
					require.NotNil(t, got.Panic)
				case "diff":
					require.NotNil(t, got.Diff)
				case "stdlib":
					require.NotNil(t, got.Assertion)
					require.Equal(t, "stdlib", got.Assertion.Library)
				}
			}
		})
	}
}

func TestRegistry_RegisterParser(t *testing.T) {
	registry := NewRegistry()
	initialCount := len(registry.Parsers())

	// create a mock parser
	mock1 := &mockParser{name: "mock"}

	// register at the beginning
	registry.RegisterParser(0, mock1)
	parsers := registry.Parsers()
	require.Len(t, parsers, initialCount+1)
	require.Equal(t, "mock", parsers[0].Name())

	// register at the end
	mock2 := &mockParser{name: "mock2"}
	registry.RegisterParser(100, mock2)
	parsers = registry.Parsers()
	require.Equal(t, "mock2", parsers[len(parsers)-1].Name())
}

func TestRegistry_Priority(t *testing.T) {
	registry := NewRegistry()

	// output that could match both testify and stdlib (testify should win)
	input := `    test.go:42: some error
Error Trace:	test.go:42
Error:      	something failed`

	got := registry.Parse(input)
	require.NotNil(t, got)
	require.Equal(t, AssertionFailure, got.FailureType)
	require.NotNil(t, got.Assertion)
	require.Equal(t, "testify", got.Assertion.Library)
}

// mockParser is a simple parser for testing
type mockParser struct {
	name string
}

func (m *mockParser) Name() string {
	return m.name
}

func (m *mockParser) CanParse(output string) bool {
	return false
}

func (m *mockParser) Parse(output string) *StructuredFailure {
	return nil
}
