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

func NewJestUI(verbose int, color bool) clio.UI {
	testRowFactory := func(ref gotest.Reference, ws tea.WindowSizeMsg) tea.Model {
		return jesttestrow.NewModel(
			ref,
			ws,
			jesttestrow.Config{
				Color:                  color,
				ShowPackages:           true,
				KeepAllTestOutput:      verbose > 0,
				KeepFailedTestOutput:   true,
				NestNonPackages:        true,
				ExpireOnCompletion:     false,
				ShowIntermediateOutput: false,
				// TODO: allow for style overrides
			},
		)
	}

	bodyHandler := pkgframe.NewFactory(testRowFactory)

	summaryHandler := jestsummary.NewFactory(
		presenter.JestTestResultSummaryConfig{
			Color:       color,
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
