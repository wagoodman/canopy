package internal

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestNewIndentWriter(t *testing.T) {
	tests := []struct {
		name           string
		ref            gotest.Reference
		expectedIndent string
	}{
		{
			name: "package reference - no indentation",
			ref: gotest.Reference{
				Package: "example/package",
			},
			expectedIndent: "",
		},
		{
			name: "function reference - no indentation",
			ref: gotest.Reference{
				Package:  "example/package",
				FuncName: "TestExample",
			},
			expectedIndent: "",
		},
		{
			name: "subtest reference - single indentation",
			ref: gotest.Reference{
				Package:  "example/package",
				FuncName: "TestExample",
				TRunName: "subtest",
			},
			expectedIndent: "    ",
		},
		{
			name: "nested subtest reference - double indentation",
			ref: gotest.Reference{
				Package:  "example/package",
				FuncName: "TestExample",
				TRunName: "parent/child",
			},
			expectedIndent: "        ",
		},
		{
			name: "deeply nested subtest reference - triple indentation",
			ref: gotest.Reference{
				Package:  "example/package",
				FuncName: "TestExample",
				TRunName: "parent/child/grandchild",
			},
			expectedIndent: "            ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := NewIndentWriter(&buf, tt.ref)

			indentWriter := writer.(*indentWriter)
			require.Equal(t, tt.expectedIndent, indentWriter.indent)
			require.Equal(t, &buf, indentWriter.w)
			require.True(t, indentWriter.atLineStart)
		})
	}
}

func TestIndentWriter_Write(t *testing.T) {
	tests := []struct {
		name           string
		ref            gotest.Reference
		input          string
		expectedOutput string
		expectedBytes  int
	}{
		{
			name: "no indentation - single line",
			ref: gotest.Reference{
				Package: "example/package",
			},
			input:          "hello world",
			expectedOutput: "hello world",
			expectedBytes:  11,
		},
		{
			name: "no indentation - multiple lines",
			ref: gotest.Reference{
				Package: "example/package",
			},
			input:          "line 1\nline 2\nline 3",
			expectedOutput: "line 1\nline 2\nline 3",
			expectedBytes:  20,
		},
		{
			name: "single level indentation - single line",
			ref: gotest.Reference{
				Package:  "example/package",
				FuncName: "TestExample",
				TRunName: "subtest",
			},
			input:          "hello world",
			expectedOutput: "    hello world",
			expectedBytes:  11,
		},
		{
			name: "single level indentation - multiple lines",
			ref: gotest.Reference{
				Package:  "example/package",
				FuncName: "TestExample",
				TRunName: "subtest",
			},
			input:          "line 1\nline 2\nline 3",
			expectedOutput: "    line 1\n    line 2\n    line 3",
			expectedBytes:  20,
		},
		{
			name: "double level indentation - multiple lines",
			ref: gotest.Reference{
				Package:  "example/package",
				FuncName: "TestExample",
				TRunName: "parent/child",
			},
			input:          "line 1\nline 2",
			expectedOutput: "        line 1\n        line 2",
			expectedBytes:  13,
		},
		{
			name: "empty input",
			ref: gotest.Reference{
				Package:  "example/package",
				FuncName: "TestExample",
				TRunName: "subtest",
			},
			input:          "",
			expectedOutput: "",
			expectedBytes:  0,
		},
		{
			name: "single newline",
			ref: gotest.Reference{
				Package:  "example/package",
				FuncName: "TestExample",
				TRunName: "subtest",
			},
			input:          "\n",
			expectedOutput: "    \n",
			expectedBytes:  1,
		},
		{
			name: "multiple newlines",
			ref: gotest.Reference{
				Package:  "example/package",
				FuncName: "TestExample",
				TRunName: "subtest",
			},
			input:          "\n\n\n",
			expectedOutput: "    \n    \n    \n",
			expectedBytes:  3,
		},
		{
			name: "text ending with newline",
			ref: gotest.Reference{
				Package:  "example/package",
				FuncName: "TestExample",
				TRunName: "subtest",
			},
			input:          "hello\nworld\n",
			expectedOutput: "    hello\n    world\n",
			expectedBytes:  12,
		},
		{
			name: "text starting with newline",
			ref: gotest.Reference{
				Package:  "example/package",
				FuncName: "TestExample",
				TRunName: "subtest",
			},
			input:          "\nhello\nworld",
			expectedOutput: "    \n    hello\n    world",
			expectedBytes:  12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := NewIndentWriter(&buf, tt.ref)

			n, err := writer.Write([]byte(tt.input))
			require.NoError(t, err)
			require.Equal(t, tt.expectedBytes, n)
			require.Equal(t, tt.expectedOutput, buf.String())
		})
	}
}

func TestIndentWriter_Write_ErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		ref       gotest.Reference
		input     string
		failAfter int // fail after this many write calls
		wantErr   require.ErrorAssertionFunc
	}{
		{
			name: "underlying writer error on indent write",
			ref: gotest.Reference{
				Package:  "example/package",
				FuncName: "TestExample",
				TRunName: "subtest",
			},
			input:     "hello",
			failAfter: 1, // fail on first write (indent)
			wantErr:   require.Error,
		},
		{
			name: "underlying writer error on content write",
			ref: gotest.Reference{
				Package:  "example/package",
				FuncName: "TestExample",
				TRunName: "subtest",
			},
			input:     "hello",
			failAfter: 2, // fail on second write (content)
			wantErr:   require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failingWriter := &failingWriter{failAfter: tt.failAfter}
			writer := NewIndentWriter(failingWriter, tt.ref)

			_, err := writer.Write([]byte(tt.input))
			tt.wantErr(t, err)
		})
	}
}

func TestIndentWriter_Write_MultipleWrites(t *testing.T) {
	var buf bytes.Buffer
	ref := gotest.Reference{
		Package:  "example/package",
		FuncName: "TestExample",
		TRunName: "subtest",
	}
	writer := NewIndentWriter(&buf, ref)

	// write multiple times to test state management
	writes := []string{"hello", " ", "world", "\n", "second", " ", "line"}
	expectedOutput := "    hello world\n    second line"

	totalBytes := 0
	for _, write := range writes {
		n, err := writer.Write([]byte(write))
		require.NoError(t, err)
		totalBytes += n
	}

	require.Equal(t, expectedOutput, buf.String())
	require.Equal(t, 23, totalBytes) // sum of input lengths: 5+1+5+1+6+1+4 = 23
}

// failingWriter is a test helper that fails after a certain number of writes
type failingWriter struct {
	writeCount int
	failAfter  int
}

func (fw *failingWriter) Write(p []byte) (n int, err error) {
	fw.writeCount++
	if fw.writeCount >= fw.failAfter {
		return 0, io.ErrShortWrite
	}
	return len(p), nil
}
