package gosummary

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/go-partybus"
)

func TestModel_RunningUntilRunEnds(t *testing.T) {
	// between packages every reference seen so far has concluded (the next package is still compiling), which
	// must not read as a final PASS until the run-end event arrives
	id := uuid.New()
	var m tea.Model = NewModel(presenter.DefaultGoTestResultSummaryConfig().WithColor(false), state.Common{}, id, gotest.RunnerConfig{})

	for _, e := range passingPackageEvents(id) {
		m, _ = m.Update(partybus.Event{Type: event.GoTestType, Value: e})
	}
	require.NotContains(t, m.View(), "PASS")

	m, _ = m.Update(partybus.Event{Type: event.GoTestRunType, Value: gotest.Run{ID: id}})
	require.Contains(t, m.View(), "PASS")
}

func TestModel_CombinedRunsRunningUntilAllRunsEnd(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	cfg := presenter.DefaultGoTestResultSummaryConfig().WithColor(false).WithCombineMultipleRuns(true)
	var m tea.Model = NewModel(cfg, state.Common{}, first, gotest.RunnerConfig{})

	m, _ = m.Update(partybus.Event{Type: event.GoTestRunRequestType, Value: gotest.RunnerConfig{}, Source: second})
	for _, e := range passingPackageEvents(first) {
		m, _ = m.Update(partybus.Event{Type: event.GoTestType, Value: e})
	}

	m, _ = m.Update(partybus.Event{Type: event.GoTestRunType, Value: gotest.Run{ID: first}})
	require.NotContains(t, m.View(), "PASS") // the second run has not ended yet

	secondRun := gotest.Run{ID: second, Result: *gotest.NewResult(gotest.ResultConfig{})}
	for _, e := range passingPackageEvents(second) {
		secondRun.Result.Update(e)
	}
	m, _ = m.Update(partybus.Event{Type: event.GoTestRunType, Value: secondRun})
	require.Contains(t, m.View(), "PASS")
}

// passingPackageEvents is a package with a single passing test, fully concluded
func passingPackageEvents(runID uuid.UUID) []gotest.Event {
	now := time.Now()
	pkg := gotest.Reference{Package: "pkg/a"}
	test := gotest.Reference{Package: "pkg/a", FuncName: "TestA"}
	return []gotest.Event{
		{RunID: runID, Time: now, Action: gotest.StartAction, Reference: pkg},
		{RunID: runID, Time: now, Action: gotest.RunAction, Reference: test},
		{RunID: runID, Time: now, Action: gotest.PassAction, Reference: test},
		{RunID: runID, Time: now, Action: gotest.PassAction, Reference: pkg},
	}
}
