package group

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter_Write(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "Test Group", GitHub)

	n, err := w.Write([]byte("line 1\n"))
	require.NoError(t, err)
	assert.Equal(t, 7, n)

	n, err = w.Write([]byte("line 2\n"))
	require.NoError(t, err)
	assert.Equal(t, 7, n)

	// buffer should be empty before flush
	assert.Empty(t, buf.String())
	assert.True(t, w.HasContent())

	// flush should write with group markers
	_, err = w.Flush()
	require.NoError(t, err)

	expected := "::group::Test Group\nline 1\nline 2\n::endgroup::\n"
	assert.Equal(t, expected, buf.String())
	assert.False(t, w.HasContent())
}

func TestWriter_WriteString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "String Test", GitHub)

	_, _ = w.WriteString("hello world\n")
	_, _ = w.WriteString("another line\n")
	_, _ = w.Flush()

	expected := "::group::String Test\nhello world\nanother line\n::endgroup::\n"
	assert.Equal(t, expected, buf.String())
}

func TestWriter_Azure(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "Azure Test", Azure)

	_, _ = w.WriteString("azure content\n")
	_, _ = w.WriteString("more content\n")
	_, _ = w.Flush()

	expected := "##[group]Azure Test\nazure content\nmore content\n##[endgroup]\n"
	assert.Equal(t, expected, buf.String())
}

func TestWriter_GitLab(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "GitLab Test", GitLab)

	_, _ = w.WriteString("gitlab content\n")
	_, _ = w.WriteString("more content\n")
	_, _ = w.Flush()

	// GitLab output contains timestamps and section IDs, so just check key parts
	output := buf.String()
	assert.Contains(t, output, "section_start:")
	assert.Contains(t, output, "[collapsed=true]")
	assert.Contains(t, output, "GitLab Test")
	assert.Contains(t, output, "gitlab content")
	assert.Contains(t, output, "section_end:")
}

func TestWriter_NilFormatter(t *testing.T) {
	var buf bytes.Buffer
	// nil formatter should default to noop
	w := NewWriter(&buf, "Nil Test", nil)

	_, _ = w.WriteString("plain content\n")
	_, _ = w.Flush()

	// content should be output without any markers
	expected := "plain content\n"
	assert.Equal(t, expected, buf.String())
}

func TestWriter_NoopFormatter(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "noop Test", noop)

	_, _ = w.WriteString("plain content\n")
	_, _ = w.Flush()

	expected := "plain content\n"
	assert.Equal(t, expected, buf.String())
}

func TestWriter_EmptyFlush(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "Empty", GitHub)

	// flush with no content should write nothing
	n, err := w.Flush()
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Empty(t, buf.String())
}

func TestWriter_Reset(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, "Reset Test", GitHub)

	_, _ = w.WriteString("content to discard\n")
	assert.True(t, w.HasContent())

	w.Reset()

	assert.False(t, w.HasContent())
	n, _ := w.Flush()
	assert.Equal(t, 0, n)
	assert.Empty(t, buf.String())
}

func TestWriter_SingleLineSkipsGrouping(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "no newline",
			content: "single line without newline",
			want:    "single line without newline",
		},
		{
			name:    "single trailing newline",
			content: "single line with newline\n",
			want:    "single line with newline\n",
		},
		{
			name:    "two lines gets grouped",
			content: "line one\nline two\n",
			want:    "::group::Test\nline one\nline two\n::endgroup::\n",
		},
		{
			name:    "multiple lines gets grouped",
			content: "line one\nline two\nline three\n",
			want:    "::group::Test\nline one\nline two\nline three\n::endgroup::\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewWriter(&buf, "Test", GitHub)

			_, _ = w.WriteString(tt.content)
			_, err := w.Flush()

			require.NoError(t, err)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}
