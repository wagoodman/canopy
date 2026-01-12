package ui

import (
	"io"

	"github.com/wagoodman/canopy/cmd/canopy/internal/cienv"
)

// TestUIConfig holds configuration for test output UI formatters, controlling visual presentation and behavior.
type TestUIConfig struct {
	// Color enables colorized output in test results.
	Color bool
	// Verbose controls the verbosity level of test output (0 = quiet, higher = more verbose).
	Verbose int
	// ShowPackagesWithNoTests controls whether to display packages that have no test files.
	ShowPackagesWithNoTests bool
	// StripPackagePrefix removes this prefix from package names in output (typically the module path).
	StripPackagePrefix string
	// Writer is the output destination (file or stdout) for this UI.
	Writer io.WriteCloser
	// IsTTY indicates whether the output destination is a terminal (affects formatting).
	IsTTY bool
	// CombineMultipleRuns controls whether to show a single summary for multiple test run sessions.
	CombineMultipleRuns bool
	// CIGrouping configures collapsible output groups for CI environments.
	CIGrouping cienv.GroupConfig
}

// DefaultTestUIConfig returns a configuration with sensible defaults (color enabled, quiet mode, no package filtering).
func DefaultTestUIConfig() TestUIConfig {
	return TestUIConfig{
		Color:                   true,
		Verbose:                 0,
		ShowPackagesWithNoTests: false,
		StripPackagePrefix:      "",
		CIGrouping:              cienv.DefaultGroupConfig(),
	}
}
