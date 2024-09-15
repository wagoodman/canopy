package studio

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/event"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
)

type controller struct {
	runner   state.RunController
	current  state.RunViewer
	selected []gotest.Reference
}

func newController(runner state.RunController) *controller {
	return &controller{
		runner: runner,
	}
}

func (c *controller) updateTestRun(tr *gotest.Run) {
	c.current = state.NewRunViewer(tr)
}

func (c *controller) updateSelection(references []gotest.Reference) {
	c.selected = references
}

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
		r, _ := c.runner.StartTests(ctx, test.RunConfig{
			LogTestFailuresAsErrors: false,
			Runner:                  cfg,
			Result: gotest.ResultConfig{
				TrackOtherOutput:   true,
				TrackFailingOutput: true,
			},
		})

		// debug.SetLine("starting testing...")

		return event.SwitchTestRun{
			TestRun: r,
		}
	}

	// TODO: add tick CMD while we are still running... until the tests have passed...
}

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
			panic("errg, no runs?") // TODO: handle this better
		} // TODO: handle this better

		run, err := config.RunStore.GetRun(latestRun.UUID)
		if err != nil {
			panic(err) // TODO: handle this better
		} // TODO: handle this better

		// TODO: if result is nil then we should have a prototype result to build on with events? No ... we're always starting with a result

		if run == nil {
			panic("errg, no run cfg?") // TODO: handle this better
		}

		return event.SwitchTestRun{
			TestRun: run,
		}
	}
}
