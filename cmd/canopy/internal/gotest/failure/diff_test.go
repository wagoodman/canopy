package failure

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestDiffParser_CanParse(t *testing.T) {
	parser := &diffParser{}

	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{
			name:   "cmp diff want got",
			input:  "(-want +got):\n-foo\n+bar",
			expect: true,
		},
		{
			name:   "cmp diff got want",
			input:  "(-got +want):\n-actual\n+expected",
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
			name:   "normal output",
			input:  "test passed",
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

func TestDiffParser_Parse(t *testing.T) {
	parser := &diffParser{}

	tests := []struct {
		name   string
		input  string
		expect *StructuredFailure
	}{
		{
			name: "simple diff",
			input: `test_file.go:42: mismatch (-want +got):
  {Name: "test",
-  Age:  25,
+  Age:  30,
  }`,
			expect: &StructuredFailure{
				FailureType: DiffFailure,
				Diff: &DiffInfo{
					Version: DiffInfoVersion,
					Chunks: []DiffChunks{
						{Type: DiffChunkContext, Content: `{Name: "test",`},
						{Type: DiffChunkRemoval, Content: "Age:  25,"},
						{Type: DiffChunkAddition, Content: "Age:  30,"},
						{Type: DiffChunkContext, Content: "}"},
					},
				},
				Location: SourceLocation{
					File: "test_file.go",
					Line: 42,
				},
			},
		},
		{
			name: "multiple changes preserves order",
			input: `(-want +got):
- old value 1
- old value 2
+ new value 1
+ new value 2
  unchanged line`,
			expect: &StructuredFailure{
				FailureType: DiffFailure,
				Diff: &DiffInfo{
					Version: DiffInfoVersion,
					Chunks: []DiffChunks{
						{Type: DiffChunkRemoval, Content: "old value 1"},
						{Type: DiffChunkRemoval, Content: "old value 2"},
						{Type: DiffChunkAddition, Content: "new value 1"},
						{Type: DiffChunkAddition, Content: "new value 2"},
						{Type: DiffChunkContext, Content: "unchanged line"},
					},
				},
			},
		},
		{
			name: "interleaved changes",
			input: `file.go:100: comparison failed (-want +got):
  &Config{
-   Host:    "localhost",
+   Host:    "127.0.0.1",
    Timeout: 30,
-   Port:    8080,
+   Port:    3000,
  }`,
			expect: &StructuredFailure{
				FailureType: DiffFailure,
				Diff: &DiffInfo{
					Version: DiffInfoVersion,
					Chunks: []DiffChunks{
						{Type: DiffChunkContext, Content: "&Config{"},
						{Type: DiffChunkRemoval, Content: `Host:    "localhost",`},
						{Type: DiffChunkAddition, Content: `Host:    "127.0.0.1",`},
						{Type: DiffChunkContext, Content: "Timeout: 30,"},
						{Type: DiffChunkRemoval, Content: "Port:    8080,"},
						{Type: DiffChunkAddition, Content: "Port:    3000,"},
						{Type: DiffChunkContext, Content: "}"},
					},
				},
				Location: SourceLocation{
					File: "file.go",
					Line: 100,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.Parse(tt.input)
			require.NotNil(t, got)

			require.Equal(t, tt.expect.FailureType, got.FailureType)
			require.NotNil(t, got.Diff)
			require.Equal(t, tt.expect.Diff.Version, got.Diff.Version)

			// compare lines in order
			if diff := cmp.Diff(tt.expect.Diff.Chunks, got.Diff.Chunks); diff != "" {
				t.Errorf("lines mismatch (-want +got):\n%s", diff)
			}

			// check location if expected
			if !tt.expect.Location.IsZero() {
				require.Equal(t, tt.expect.Location.File, got.Location.File)
				require.Equal(t, tt.expect.Location.Line, got.Location.Line)
			}

			// verify raw output preserved
			require.Equal(t, tt.input, got.RawOutput)
		})
	}
}
