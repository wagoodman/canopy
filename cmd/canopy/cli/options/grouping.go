package options

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/group"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ci"

	"github.com/anchore/clio"
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
	// Skipped controls whether skipped output is grouped (collapsed by default).
	// This includes packages with no test files.
	Skipped bool `yaml:"skipped" json:"skipped" mapstructure:"skipped"`
	// AcrossTests groups consecutive test conclusions within a package when their
	// status matches an enabled grouping option (passed, failed, or skipped).
	// This helps reduce noise when a package has many passing/skipped tests and a few failures.
	AcrossTests bool `yaml:"across-tests" json:"across-tests" mapstructure:"across-tests"`
	// AcrossPackages groups consecutive packages together when their status matches
	// an enabled grouping option (passed, failed, or skipped). This reduces noise when
	// there are many passing/skipped packages before a failure.
	AcrossPackages bool `yaml:"across-packages" json:"across-packages" mapstructure:"across-packages"`
	// AcrossCases groups consecutive subtests/cases within a parent test when the parent
	// has at least one child that should NOT be grouped (e.g., a failure). This keeps
	// failures visible while collapsing passing subtests around them.
	AcrossCases bool `yaml:"across-cases" json:"across-cases" mapstructure:"across-cases"`
}

// DefaultGrouping returns the default grouping configuration.
// By default, grouping is auto-detected from the CI environment, passed and skipped output
// is grouped, and failed output is not grouped (so failures are immediately visible).
func DefaultGrouping() Grouping {
	return Grouping{
		Style:          "auto", // resolved in PostLoad
		Passed:         true,
		Failed:         false,
		Skipped:        true,
		AcrossTests:    true,
		AcrossPackages: true,
		AcrossCases:    true,
	}
}

func (o *Grouping) DescribeFields(descriptions clio.FieldDescriptionSet) {
	descriptions.Add(&o.Style, "the CI grouping style to use: auto (detect from environment), github, gitlab, azure, off")
	descriptions.Add(&o.Passed, "whether to group passed output (collapsed by default)")
	descriptions.Add(&o.Failed, "whether to group failed output")
	descriptions.Add(&o.Skipped, "whether to group skipped output (collapsed by default)")
	descriptions.Add(&o.AcrossTests, "whether to group consecutive test conclusions within a package based on enabled status types")
	descriptions.Add(&o.AcrossPackages, "whether to group consecutive packages together based on enabled status types")
	descriptions.Add(&o.AcrossCases, "whether to group consecutive subtests/cases within a failing parent test based on enabled status types")
}

// ToAPIConfig converts Grouping to a group.Config for use with the grouping writer.
func (o Grouping) ToAPIConfig() group.Config {
	return group.Config{
		Formatter:      styleToFormatter(o.Style),
		GroupPassed:    o.Passed,
		GroupFailed:    o.Failed,
		GroupSkipped:   o.Skipped,
		AcrossTests:    o.AcrossTests,
		AcrossPackages: o.AcrossPackages,
		AcrossCases:    o.AcrossCases,
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
