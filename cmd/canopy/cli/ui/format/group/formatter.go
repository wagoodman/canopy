package group

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Formatter formats a title and content into a grouped output string.
type Formatter func(title, content string) string

// sectionCounter generates unique section IDs for GitLab CI.
var sectionCounter uint64

// noop returns content unchanged (no grouping).
func noop(_, content string) string {
	return content
}

// GitHub formats for GitHub Actions collapsible groups.
func GitHub(title, content string) string {
	return fmt.Sprintf("::group::%s\n%s::endgroup::\n", title, content)
}

// GitLab formats for GitLab CI collapsible sections.
func GitLab(title, content string) string {
	ts := time.Now().Unix()
	id := atomic.AddUint64(&sectionCounter, 1)
	name := fmt.Sprintf("section_%d", id)
	return fmt.Sprintf("\x1b[0Ksection_start:%d:%s[collapsed=true]\r\x1b[0K%s\n%s\x1b[0Ksection_end:%d:%s\r\x1b[0K\n",
		ts, name, title, content, ts, name)
}

// Azure formats for Azure Pipelines collapsible groups.
func Azure(title, content string) string {
	return fmt.Sprintf("##[group]%s\n%s##[endgroup]\n", title, content)
}

// StreamingMarkers holds pre-computed start and end markers for streaming output.
// This ensures that section IDs match between start and end (important for GitLab).
type StreamingMarkers struct {
	Start string
	End   string
}

// NewStreamingMarkers creates start/end markers for streaming output from a formatter.
// The markers are computed once to ensure consistency (e.g., GitLab section IDs match).
func NewStreamingMarkers(formatter Formatter, title string) StreamingMarkers {
	if formatter == nil {
		return StreamingMarkers{}
	}

	// use the formatter with a placeholder to get the full format,
	// then split into start and end portions
	const placeholder = "\x00CONTENT\x00"
	full := formatter(title, placeholder)

	// find the placeholder and split
	idx := -1
	for i := 0; i <= len(full)-len(placeholder); i++ {
		if full[i:i+len(placeholder)] == placeholder {
			idx = i
			break
		}
	}

	if idx == -1 {
		// placeholder not found - formatter doesn't include content (noop case)
		return StreamingMarkers{}
	}

	return StreamingMarkers{
		Start: full[:idx],
		End:   full[idx+len(placeholder):],
	}
}
