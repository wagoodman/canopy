package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/dottestrow"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/jestsummary"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/pkgframe"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/syncspinner"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"

	"github.com/anchore/clio"
)

func NewTestDotUI(config TestUIConfig) clio.UI {
	rowCfg := dottestrow.Config{
		Color:                  config.Color,
		ShowPackages:           true,
		KeepFailedTestOutput:   true,
		NestNonPackages:        true,
		ExpireOnCompletion:     false,
		ShowIntermediateOutput: false,
		// TODO: allow for style overrides
	}

	spin := syncspinner.New()

	common := state.Common{
		Spinner: spin.CurrentTick(),
	}

	pkgModelFactory := func(e gotest.Event, common state.Common) tea.Model {
		return dottestrow.NewModel(e.Reference, common, rowCfg)
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
		WithSimpleUI(newSimpleUI().
			withNotifications().
			withReports(),
		).
		WithSyncSpinner(spin).
		WithFooter(summaryHandler)

	return NewTeaUI(c)
}
