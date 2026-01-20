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
