package ui

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/adapter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"

	"github.com/anchore/clio"
)

// TestNoUI creates a minimal UI that only handles notifications and reports without a terminal interface.
// This is suitable for non-interactive scenarios or when the primary output goes to a file.
func TestNoUI() clio.UI {
	return newCoreUI().
		withNotifications().
		withReports().
		withHandledPresenters(
			adapter.NewTestRun(presenter.JestTestResultSummaryConfig{}.New),
		)
}
