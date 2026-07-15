package studio

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/event"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
)

// controller manages test execution state and coordinates test re-runs based on
// user selections.
type controller struct {
	// runner executes test runs.
	runner state.RunController

	// current is the viewer for the currently displayed test run.
	current state.RunViewer

	// selected contains the test references selected by the user.
	selected []gotest.Reference
}

// newController creates a new controller with the given run controller.
func newController(runner state.RunController) *controller {
	return &controller{
		runner: runner,
	}
}

// updateTestRun sets the current test run being viewed.
func (c *controller) updateTestRun(tr *gotest.Run) {
	c.current = state.NewRunViewer(tr)
}

// updateSelection updates the set of selected test references.
func (c *controller) updateSelection(references []gotest.Reference) {
	c.selected = references
}

// startTestReRun initiates a new test run. If all is true, all tests from the
// current run are executed; otherwise only the selected tests are run. Returns
// a Bubble Tea command that switches to the new test run when complete.
func (c controller) startTestReRun(ctx context.Context, all bool) tea.Cmd {
	if c.current == nil {
		return nil
		// return func() tea.Msg {
		//	return event.ActionError{
		//		Message: "no tests selected to re-run",
		//	}
		//}
	}

	cfg := c.current.Config()
	if !all {
		// only run the selected tests
		cfg.OnlyRefs = onlyRefs(c.current, c.selected)
	} else {
		// reset test reference filters
		cfg.OnlyRefs = nil
	}

	return func() tea.Msg {
		r, errs := c.runner.StartTests(ctx, test.RunConfig{
			LogTestFailuresAsErrors: false,
			Runner:                  cfg,
			Result: gotest.ResultConfig{
				TrackOtherOutput:   true,
				TrackFailingOutput: true,
			},
		})

		if r == nil {
			// run creation failed (session/run insert error, runner failed to start); the
			// error is on errs. surface it instead of forwarding a nil run that every
			// SwitchTestRun consumer would panic dereferencing (mirrors the stored-run path).
			err := <-errs
			log.WithFields("error", err).Error("failed to start test re-run")
			msg := "failed to start test run"
			if err != nil {
				msg += ": " + err.Error()
			}
			return event.ActionError{Message: msg}
		}

		return event.SwitchTestRun{
			TestRun: r,
		}
	}

	// TODO: add tick CMD while we are still running... until the tests have passed...
}

// onlyRefs computes the minimal set of test references needed to run all the
// given refs. If refs is empty or represents all tests in the run, returns nil.
// Otherwise returns a minimized set of references using package-level or
// function-level references where possible.
func onlyRefs(run state.RunViewer, refs []gotest.Reference) []gotest.Reference {
	if len(refs) == 0 {
		return nil
	}

	isAll := true
	for _, ref := range refs {
		if len(run.ReferenceEvents(ref)) == 0 {
			isAll = false
			break
		}
	}

	if isAll {
		return nil
	}

	// we need to craft a set of references that minimally selects all given references. It might be that we've been given
	// all test functions and cases for a single package, in which case, we only need to provide the single reference that
	// represents that package.
	return gotest.MinimizeReferences(run.References(), refs)
}

// switchToLatestStoredTestRun loads and switches to the most recent test run
// from the session. Returns a Bubble Tea command that emits a SwitchTestRun event.
func (c controller) switchToLatestStoredTestRun(config Config) tea.Cmd {
	return func() tea.Msg {
		// get latest run
		var latestRun *test.RunInfo
		for i := range config.SessionInfo.Runs {
			r := config.SessionInfo.Runs[i]
			if latestRun == nil || r.Started.After(latestRun.Started) {
				latestRun = &r
			}
		}

		if latestRun == nil {
			// a session with no runs shouldn't crash the studio; surface it as an action error
			log.Warn("no test runs in session to display")
			return event.ActionError{Message: "no test runs found in this session"}
		}

		run, err := config.RunStore.GetRun(latestRun.UUID)
		if err != nil {
			log.WithFields("error", err, "run", latestRun.UUID).Error("failed to load stored test run")
			return event.ActionError{Message: "failed to load the latest test run: " + err.Error()}
		}

		if run == nil {
			log.WithFields("run", latestRun.UUID).Warn("stored test run not found")
			return event.ActionError{Message: "the latest test run could not be found"}
		}

		return event.SwitchTestRun{
			TestRun: run,
		}
	}
}
