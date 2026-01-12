package cienv

import (
	"fmt"
	"io"
	"strings"
)

// GroupWriter wraps an io.Writer to add CI-specific group markers around output.
// It buffers writes and flushes them with appropriate group commands when Flush is called.
type GroupWriter struct {
	writer  io.Writer
	title   string
	buffer  strings.Builder
	started bool
}

// NewGroupWriter creates a new GroupWriter that will wrap output in a collapsible group.
// The title is the name displayed for the collapsed section.
func NewGroupWriter(w io.Writer, title string) *GroupWriter {
	return &GroupWriter{
		writer: w,
		title:  title,
	}
}

// Write buffers content to be written when Flush is called.
func (g *GroupWriter) Write(p []byte) (n int, err error) {
	return g.buffer.Write(p)
}

// WriteString buffers a string to be written when Flush is called.
func (g *GroupWriter) WriteString(s string) (n int, err error) {
	return g.buffer.WriteString(s)
}

// Flush writes the buffered content with group markers to the underlying writer.
// If there is no buffered content, nothing is written.
// Returns the number of bytes written and any error.
func (g *GroupWriter) Flush() (int, error) {
	if g.buffer.Len() == 0 {
		return 0, nil
	}

	content := g.buffer.String()
	g.buffer.Reset()

	// Write the group start, content, and group end
	output := fmt.Sprintf("::group::%s\n%s::endgroup::\n", g.title, content)
	return g.writer.Write([]byte(output))
}

// HasContent returns true if there is buffered content to write.
func (g *GroupWriter) HasContent() bool {
	return g.buffer.Len() > 0
}

// Reset clears the buffer without writing.
func (g *GroupWriter) Reset() {
	g.buffer.Reset()
}

// GroupConfig controls how output is grouped in CI environments.
type GroupConfig struct {
	// Enabled controls whether grouping is active.
	// If nil, auto-detection based on CI environment is used.
	Enabled *bool

	// GroupPassedPackages causes passed package output to be grouped (collapsed).
	GroupPassedPackages bool

	// GroupFailedPackages causes failed package output to be grouped.
	// Note: In GitHub Actions, failed groups may auto-expand.
	GroupFailedPackages bool
}

// DefaultGroupConfig returns the default grouping configuration.
// By default, passed packages are grouped, failed packages are not.
func DefaultGroupConfig() GroupConfig {
	return GroupConfig{
		Enabled:             nil, // auto-detect
		GroupPassedPackages: true,
		GroupFailedPackages: false,
	}
}

// IsEnabled returns whether grouping should be enabled based on the config
// and the detected CI environment.
func (c GroupConfig) IsEnabled() bool {
	return c.IsEnabledWith(Detect)
}

// IsEnabledWith returns whether grouping should be enabled, using a custom detector.
func (c GroupConfig) IsEnabledWith(detect func() *Environment) bool {
	// Explicit configuration takes precedence
	if c.Enabled != nil {
		return *c.Enabled
	}

	// Auto-detect: enable if in a CI that supports grouping
	env := detect()
	return env != nil && env.SupportsGrouping
}

// ShouldGroup returns whether the given package result should be grouped.
func (c GroupConfig) ShouldGroup(passed bool) bool {
	if passed {
		return c.GroupPassedPackages
	}
	return c.GroupFailedPackages
}
