// Package group provides output grouping formatters for collapsible sections
// in CI environments like GitHub Actions, GitLab CI, and Azure Pipelines.
package group

// Config controls how output is grouped.
type Config struct {
	// Formatter is the formatting function to use for grouping.
	// If nil, grouping is disabled.
	Formatter Formatter

	// GroupPassed causes passed output to be grouped (collapsed).
	GroupPassed bool

	// GroupFailed causes failed output to be grouped.
	GroupFailed bool
}

// ShouldGroup returns whether output should be grouped based on pass/fail status.
func (c Config) ShouldGroup(passed bool) bool {
	if c.Formatter == nil {
		return false
	}
	if passed {
		return c.GroupPassed
	}
	return c.GroupFailed
}
