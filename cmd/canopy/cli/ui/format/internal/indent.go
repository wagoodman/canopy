package internal

import (
	"io"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

type indentWriter struct {
	w           io.Writer
	indent      string
	atLineStart bool
}

func NewIndentWriter(w io.Writer, ref gotest.Reference) io.Writer {
	var count int
	if ref.IsSubTest() {
		count = strings.Count(ref.TRunName, "/") + 1
	}

	return &indentWriter{
		w:           w,
		indent:      strings.Repeat("    ", count),
		atLineStart: true,
	}
}

func (iw *indentWriter) Write(p []byte) (int, error) {
	var written int
	for i, b := range p {
		if iw.atLineStart {
			n, err := iw.w.Write([]byte(iw.indent))
			if err != nil {
				return written, err
			}
			written += n
			iw.atLineStart = false
		}

		n, err := iw.w.Write(p[i : i+1])
		if err != nil {
			return written, err
		}
		written += n

		if b == '\n' {
			iw.atLineStart = true
		}
	}
	return len(p), nil
}
