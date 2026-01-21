// Package group provides output grouping formatters for collapsible sections
// in CI environments like GitHub Actions, GitLab CI, and Azure Pipelines.
package group

import (
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// Config controls how output is grouped.
type Config struct {
	// Formatter is the formatting function to use for grouping.
	// If nil, grouping is disabled.
	Formatter Formatter

	// GroupPassed causes passed output to be grouped (collapsed).
	GroupPassed bool

	// GroupFailed causes failed output to be grouped.
	GroupFailed bool

	// GroupSkipped causes skipped output to be grouped (collapsed).
	GroupSkipped bool

	// AcrossTests groups consecutive test conclusions within a package when their
	// status matches an enabled grouping option (passed, failed, or skipped).
	// This helps reduce noise when a package has many passing/skipped tests and a few failures.
	AcrossTests bool

	// AcrossPackages groups consecutive packages together when their status matches
	// an enabled grouping option (passed, failed, or skipped). This reduces noise when
	// there are many passing/skipped packages before a failure.
	AcrossPackages bool
}

// ShouldGroup returns whether output should be grouped based on the action status.
func (c Config) ShouldGroup(action gotest.Action) bool {
	if c.Formatter == nil {
		return false
	}
	switch action {
	case gotest.PassAction:
		return c.GroupPassed
	case gotest.FailAction:
		return c.GroupFailed
	case gotest.SkipAction:
		return c.GroupSkipped
	default:
		return false
	}
}

// GroupedStatusLabel returns a label describing which statuses are being grouped.
// For example: "passed", "passed/skipped", "passed/failed/skipped"
func (c Config) GroupedStatusLabel() string {
	var parts []string
	if c.GroupPassed {
		parts = append(parts, "passed")
	}
	if c.GroupFailed {
		parts = append(parts, "failed")
	}
	if c.GroupSkipped {
		parts = append(parts, "skipped")
	}
	return strings.Join(parts, "/")
}
