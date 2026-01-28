package failure

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "memory address",
			input: "pointer at 0xc0000a4000",
			want:  "pointer at <addr>",
		},
		{
			name:  "multiple addresses",
			input: "from 0xc000012340 to 0xc000056789",
			want:  "from <addr> to <addr>",
		},
		{
			name:  "timestamp RFC3339",
			input: "at 2024-01-15T10:30:45",
			want:  "at <timestamp>",
		},
		{
			name:  "timestamp with space",
			input: "logged 2024-01-15 10:30:45",
			want:  "logged <timestamp>",
		},
		{
			name:  "nanosecond time",
			input: "at 10:30:45.123456789",
			want:  "at <time>",
		},
		{
			name:  "uuid",
			input: "id: 550e8400-e29b-41d4-a716-446655440000",
			want:  "id: <uuid>",
		},
		{
			name:  "goroutine id",
			input: "goroutine 42 [running]",
			want:  "goroutine <id> [running]",
		},
		{
			name:  "no changes needed",
			input: "simple message",
			want:  "simple message",
		},
		{
			name:  "whitespace trimmed",
			input: "  padded message  ",
			want:  "padded message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeValue(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestComputeFingerprint(t *testing.T) {
	tests := []struct {
		name     string
		failure  *StructuredFailure
		wantNil  bool
		wantSame bool // if true, check fingerprint matches another case
		sameAs   string
	}{
		{
			name:    "nil failure",
			failure: nil,
			wantNil: true,
		},
		{
			name: "assertion failure",
			failure: &StructuredFailure{
				FailureType: AssertionFailure,
				Assertion: &AssertionInfo{
					Library:  "testify",
					Function: "Equal",
					Expected: "foo",
					Actual:   "bar",
				},
				Location: SourceLocation{
					File: "test.go",
					Line: 42,
				},
			},
		},
		{
			name: "same assertion different line",
			failure: &StructuredFailure{
				FailureType: AssertionFailure,
				Assertion: &AssertionInfo{
					Library:  "testify",
					Function: "Equal",
					Expected: "foo",
					Actual:   "bar",
				},
				Location: SourceLocation{
					File: "test.go",
					Line: 100, // different line
				},
			},
			wantSame: true,
			sameAs:   "assertion failure",
		},
		{
			name: "panic failure",
			failure: &StructuredFailure{
				FailureType: PanicFailure,
				Panic: &PanicInfo{
					Message: "nil pointer dereference",
					Frames: []StackFrame{
						{Function: "main.handleRequest", File: "handler.go", Line: 42, IsUser: true},
						{Function: "runtime.gopanic", File: "panic.go", Line: 1000, IsUser: false},
					},
				},
			},
		},
		{
			name: "diff failure",
			failure: &StructuredFailure{
				FailureType: DiffFailure,
				Diff: &DiffInfo{
					Version: DiffInfoVersion,
					Chunks: []DiffChunks{
						{Type: DiffChunkRemoval, Content: "bar"},
						{Type: DiffChunkAddition, Content: "foo"},
					},
				},
			},
		},
		{
			name: "unknown failure",
			failure: &StructuredFailure{
				FailureType: UnknownFailure,
				RawOutput:   "some error occurred",
			},
		},
	}

	fingerprints := make(map[string]string)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeFingerprint(tt.failure)

			if tt.wantNil {
				require.Empty(t, got)
				return
			}

			require.NotEmpty(t, got)
			require.Len(t, got, 32) // 16 bytes = 32 hex chars

			if tt.wantSame {
				require.Equal(t, fingerprints[tt.sameAs], got)
			} else {
				fingerprints[tt.name] = got
			}
		})
	}

	// verify different failure types produce different fingerprints
	require.NotEqual(t, fingerprints["assertion failure"], fingerprints["panic failure"])
	require.NotEqual(t, fingerprints["assertion failure"], fingerprints["diff failure"])
	require.NotEqual(t, fingerprints["panic failure"], fingerprints["diff failure"])
}

func TestComputeFingerprint_Stability(t *testing.T) {
	// verify that the same input produces the same fingerprint
	failure := &StructuredFailure{
		FailureType: AssertionFailure,
		Assertion: &AssertionInfo{
			Library:  "testify",
			Function: "Equal",
			Expected: "expected value",
			Actual:   "actual value",
		},
	}

	fp1 := ComputeFingerprint(failure)
	fp2 := ComputeFingerprint(failure)
	require.Equal(t, fp1, fp2)
}
