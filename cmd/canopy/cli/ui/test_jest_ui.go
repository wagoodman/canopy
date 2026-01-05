package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/jestsummary"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/jesttestrow"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/pkgframe"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/syncspinner"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"

	"github.com/anchore/clio"
)

// NewTestJestUI creates a new UI for displaying test results in a Jest-style format.
// Today this is experimental thus requires opting into via configuration.
func NewTestJestUI(config TestUIConfig) clio.UI {
	rowCfg := jesttestrow.Config{
		Color:                       config.Color,
		ShowPackages:                true,
		KeepAllTestOutput:           config.Verbose > 0,
		KeepFailedTestOutput:        true,
		NestNonPackages:             true,
		ExpireOnCompletion:          false,
		ShowIntermediateOutput:      false,
		HidePackagesWithNoTestFiles: !config.ShowPackagesWithNoTests,
		// TODO: allow for style overrides
	}

	spin := syncspinner.New()

	common := state.Common{
		Spinner: spin.CurrentTick(),
	}

	testRowFactory := func(e gotest.Event, common state.Common) tea.Model {
		return jesttestrow.NewModel(e.Reference, common, rowCfg)
	}

	pkgModelFactory := func(e gotest.Event, common state.Common) tea.Model {
		return pkgframe.NewPackageModel(e.Reference, common, testRowFactory)
	}

	bodyHandler := pkgframe.NewFactory(
		pkgModelFactory,
		pkgframe.FactoryConfig{
			ShowPackagesMissingTests: config.ShowPackagesWithNoTests,
			Common:                   common,
		},
	)

	summaryHandler := jestsummary.NewFactory(
		presenter.JestTestResultSummaryConfig{
			Color:       config.Color,
			ShowElapsed: true,
		},
	)

	c := NewTeaUIConfig(bodyHandler).
		WithCoreUI(newCoreUI().
			withNotifications().
			withReports(),
		).
		WithSyncSpinner(spin).
		WithFooter(summaryHandler)

	return NewTeaUI(c)
}
