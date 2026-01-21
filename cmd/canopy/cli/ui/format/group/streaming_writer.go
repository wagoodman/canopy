package group

import "io"

// StreamingGroupWriter writes group content incrementally without buffering.
// Unlike Writer which buffers all content before emitting group markers,
// StreamingGroupWriter emits the start marker immediately on Start() and
// passes writes directly to the underlying writer.
type StreamingGroupWriter struct {
	writer  io.Writer
	markers StreamingMarkers
	started bool
}

// NewStreamingGroupWriter creates a new streaming writer for grouping output.
// If formatter is nil, content passes through unchanged (no group markers).
func NewStreamingGroupWriter(w io.Writer, title string, formatter Formatter) *StreamingGroupWriter {
	return &StreamingGroupWriter{
		writer:  w,
		markers: NewStreamingMarkers(formatter, title),
	}
}

// Start writes the group header immediately.
// Subsequent calls are no-ops until End() is called.
func (s *StreamingGroupWriter) Start() error {
	if s.started {
		return nil
	}
	s.started = true
	if s.markers.Start == "" {
		return nil
	}
	_, err := s.writer.Write([]byte(s.markers.Start))
	return err
}

// Write passes content directly to the underlying writer (no buffering).
func (s *StreamingGroupWriter) Write(p []byte) (n int, err error) {
	return s.writer.Write(p)
}

// End writes the group footer.
// Only writes if the group was started.
func (s *StreamingGroupWriter) End() error {
	if !s.started {
		return nil
	}
	s.started = false
	if s.markers.End == "" {
		return nil
	}
	_, err := s.writer.Write([]byte(s.markers.End))
	return err
}

// Started returns whether the group has been started.
func (s *StreamingGroupWriter) Started() bool {
	return s.started
}
