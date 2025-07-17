package ui

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/adapter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"

	"github.com/anchore/clio"
)

func TestNoUI() clio.UI {
	return newSimpleUI().
		withNotifications().
		withReports().
		withHandledPresenters(
			adapter.NewTestRun(presenter.JestTestResultSummaryConfig{}.New),
		)
}
