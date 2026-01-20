package options

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/group"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ci"
)

// Grouping configures collapsible output groups for CI environments like GitHub Actions,
// Azure Pipelines, and GitLab CI. When enabled and running in a supported CI, output
// is wrapped in collapsible groups.
type Grouping struct {
	// Style specifies the CI grouping format to use.
	// Values: "auto" (detect from environment), "github", "gitlab", "azure", "off"
	Style string `yaml:"style" json:"style" mapstructure:"style"`
	// Passed controls whether passed output is grouped (collapsed by default).
	Passed bool `yaml:"passed" json:"passed" mapstructure:"passed"`
	// Failed controls whether failed output is grouped.
	Failed bool `yaml:"failed" json:"failed" mapstructure:"failed"`
	// AcrossTests groups consecutive passing/failing test conclusions within a package,
	// even when the package itself isn't grouped. This helps reduce noise when a package
	// has many passing tests and a few failures.
	AcrossTests bool `yaml:"across-tests" json:"across-tests" mapstructure:"across-tests"`
}

// DefaultGrouping returns the default grouping configuration.
// By default, grouping is auto-detected from the CI environment, passed output is grouped,
// and failed output is not grouped (so failures are immediately visible).
func DefaultGrouping() Grouping {
	return Grouping{
		Style:       "auto", // resolved in PostLoad
		Passed:      true,
		Failed:      false,
		AcrossTests: true, // enable by default when grouping is active
	}
}

// ToAPIConfig converts Grouping to a group.Config for use with the grouping writer.
func (o Grouping) ToAPIConfig() group.Config {
	return group.Config{
		Formatter:   styleToFormatter(o.Style),
		GroupPassed: o.Passed,
		GroupFailed: o.Failed,
		AcrossTests: o.AcrossTests,
	}
}

// styleToFormatter converts a style string to a group.Formatter.
func styleToFormatter(style string) group.Formatter {
	switch style {
	case "github":
		return group.GitHub
	case "gitlab":
		return group.GitLab
	case "azure":
		return group.Azure
	default:
		return nil // disabled
	}
}

// PostLoad applies environment variable overrides (e.g., NO_COLOR) to the configuration.
func (o *Grouping) PostLoad() error {
	// resolve "auto" to concrete CI style
	if o.Style == "auto" {
		o.Style = detectCIStyle()
	}
	return nil
}

// detectCIStyle detects the CI environment and returns the appropriate style string.
func detectCIStyle() string {
	provider := ci.Detect()
	switch provider {
	case ci.ProviderGitHub:
		return "github"
	case ci.ProviderGitLab:
		return "gitlab"
	case ci.ProviderAzure:
		return "azure"
	default:
		return "off"
	}
}
