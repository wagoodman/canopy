package options

import "github.com/wagoodman/canopy/cmd/canopy/internal/bus"

// Experiment configures experimental features that are not yet stable or ready for general use.
// These features are disabled by default and can be enabled via environment variables or config file.
type Experiment struct {
	// DotUI enables the dot-style output format for test results.
	DotUI bool `yaml:"dot-ui" json:"dot-ui" mapstructure:"dot-ui"`
	// JestUI enables the Jest-style output format for test results.
	JestUI bool `yaml:"jest-ui" json:"jest-ui" mapstructure:"jest-ui"`
	// BusDebug enables debug logging for the event bus system.
	BusDebug bool `yaml:"bus-debug" json:"bus-debug" mapstructure:"bus-debug"`
}

// DefaultExperiment returns experiment options with all experimental features disabled.
func DefaultExperiment() Experiment {
	return Experiment{
		DotUI:  false,
		JestUI: false,
	}
}

func (e *Experiment) PostLoad() error {
	if e.BusDebug {
		bus.Debug(true)
	}
	return nil
}
