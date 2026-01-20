package group

import (
	"io"
	"strings"
)

// Writer wraps an io.Writer to add group markers around output.
// It buffers writes and flushes them with appropriate group formatting when Flush is called.
type Writer struct {
	writer    io.Writer
	title     string
	buffer    strings.Builder
	formatter Formatter
}

// NewWriter creates a new Writer for grouping output.
// If formatter is nil, noop is used (content passes through unchanged).
func NewWriter(w io.Writer, title string, formatter Formatter) *Writer {
	if formatter == nil {
		formatter = noop
	}
	return &Writer{
		writer:    w,
		title:     title,
		formatter: formatter,
	}
}

// Write buffers content to be written when Flush is called.
func (g *Writer) Write(p []byte) (n int, err error) {
	return g.buffer.Write(p)
}

// WriteString buffers a string to be written when Flush is called.
func (g *Writer) WriteString(s string) (n int, err error) {
	return g.buffer.WriteString(s)
}

// Flush writes the buffered content with group formatting to the underlying writer.
// If there is no buffered content, nothing is written.
// Returns the number of bytes written and any error.
func (g *Writer) Flush() (int, error) {
	if g.buffer.Len() == 0 {
		return 0, nil
	}

	output := g.formatter(g.title, g.buffer.String())
	g.buffer.Reset()
	return g.writer.Write([]byte(output))
}

// HasContent returns true if there is buffered content to write.
func (g *Writer) HasContent() bool {
	return g.buffer.Len() > 0
}

// Reset clears the buffer without writing.
func (g *Writer) Reset() {
	g.buffer.Reset()
}
