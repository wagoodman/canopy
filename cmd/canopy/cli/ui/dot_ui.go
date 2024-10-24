package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/dottestrow"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/jestsummary"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/pkgframe"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"

	"github.com/anchore/clio"
)

func NewDotUI(config Config) clio.UI {
	rowCfg := dottestrow.Config{
		Color:                  config.Color,
		ShowPackages:           true,
		KeepFailedTestOutput:   true,
		NestNonPackages:        true,
		ExpireOnCompletion:     false,
		ShowIntermediateOutput: false,
		// TODO: allow for style overrides
	}

	pkgModelFactory := func(e gotest.Event, ws tea.WindowSizeMsg) tea.Model {
		return dottestrow.NewModel(e.Reference, ws, rowCfg)
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
