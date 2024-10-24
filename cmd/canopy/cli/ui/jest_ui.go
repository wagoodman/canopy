package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/jestsummary"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/jesttestrow"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/pkgframe"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"

	"github.com/anchore/clio"
)

func NewJestUI(config Config) clio.UI {
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
	testRowFactory := func(e gotest.Event, ws tea.WindowSizeMsg) tea.Model {
		return jesttestrow.NewModel(e.Reference, ws, rowCfg)
	}

	pkgModelFactory := func(e gotest.Event, ws tea.WindowSizeMsg) tea.Model {
		return pkgframe.NewPackageModel(e.Reference, ws, testRowFactory)
	}

	bodyHandler := pkgframe.NewFactory(pkgModelFactory, config.ShowPackagesWithNoTests)

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
		WithFooter(summaryHandler)

	return NewTeaUI(c)
}
