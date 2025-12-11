package options

// Experiment configures experimental features that are not yet stable or ready for general use.
// These features are disabled by default and can be enabled via environment variables or config file.
type Experiment struct {
	// DotUI enables the dot-style output format for test results.
	DotUI bool `yaml:"dot-ui" json:"dot-ui" mapstructure:"dot-ui"`
	// JestUI enables the Jest-style output format for test results.
	JestUI bool `yaml:"jest-ui" json:"jest-ui" mapstructure:"jest-ui"`
}

// DefaultExperiment returns experiment options with all experimental features disabled.
func DefaultExperiment() Experiment {
	return Experiment{
		DotUI:  false,
		JestUI: false,
	}
}
