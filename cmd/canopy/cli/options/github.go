package options

import (
	"github.com/wagoodman/canopy/cmd/canopy/internal/cienv"
)

// GitHub configures GitHub Actions-specific features and integrations.
type GitHub struct {
	// Grouping configures collapsible output groups for GitHub Actions workflows.
	Grouping Grouping `yaml:"grouping" json:"grouping" mapstructure:"grouping"`
}

// Grouping configures collapsible output groups for CI environments like GitHub Actions.
// When enabled and running in a supported CI, package output is wrapped in collapsible groups.
type Grouping struct {
	// Enabled explicitly enables or disables CI grouping. When nil, auto-detection is used.
	Enabled *bool `yaml:"enabled" json:"enabled" mapstructure:"enabled"`
	// Passed controls whether passed output is grouped (collapsed by default).
	Passed bool `yaml:"passed" json:"passed" mapstructure:"passed"`
	// Failed controls whether failed output is grouped.
	Failed bool `yaml:"failed" json:"failed" mapstructure:"failed"`
}

// DefaultGitHub returns GitHub options with sensible defaults for CI grouping.
func DefaultGitHub() GitHub {
	return GitHub{
		Grouping: DefaultGrouping(),
	}
}

// DefaultGrouping returns the default grouping configuration.
// By default, grouping is auto-detected from the CI environment, passed output is grouped,
// and failed output is not grouped (so failures are immediately visible).
func DefaultGrouping() Grouping {
	return Grouping{
		Enabled: nil, // auto-detect
		Passed:  true,
		Failed:  false,
	}
}

// ToGroupConfig converts Grouping to a cienv.GroupConfig for use with the grouping writer.
func (g Grouping) ToGroupConfig() cienv.GroupConfig {
	return cienv.GroupConfig{
		Enabled:             g.Enabled,
		GroupPassedPackages: g.Passed,
		GroupFailedPackages: g.Failed,
	}
}
