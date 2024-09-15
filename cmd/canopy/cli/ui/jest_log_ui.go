package ui

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/jestsummary"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/jesttestrow"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"

	"github.com/anchore/clio"
)

func NewJestLogUI(verbose int, color bool) clio.UI {
	bodyHandler := jesttestrow.NewFactory(
		jesttestrow.Config{
			Color:                  color,
			ShowPackages:           true,
			KeepAllTestOutput:      verbose > 0,
			KeepFailedTestOutput:   true,
			NestNonPackages:        false,
			ExpireOnCompletion:     true,
			ShowIntermediateOutput: true,
			// TODO: allow for style overrides
		},
	)
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
