package studio

import (
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/x/ansi"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// this file is a behavioral smoke harness for the interactive studio TUI. it drives
// the real Model (and, through it, the dispatch + both panes) with scripted event
// sequences and asserts two things the interactive path is easy to regress on:
//   1. no sequence of keys/events panics (the class of bug this branch kept finding:
//      nil-run derefs, double-quit WaitGroup underflow, empty-list navigation)
//   2. the rendered View reflects the loaded run (a failing test shows up, not silently
//      swallowed)
//
// it deliberately avoids golden snapshots: the View is lipgloss-colored and
// bubblezone-wrapped, so byte-exact goldens churn on every style tweak. substring
// checks on the ansi-stripped output are enough to catch "renders nothing / lost the
// failing test" without the maintenance tax.

// key builds a KeyMsg for a rune sequence (letters, "?", "/", space, etc).
func keyMsg(s string) tea.KeyMsg {
	if s == " " {
		return tea.KeyMsg{Type: tea.KeySpace}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// newTestModel builds a studio Model wired to a WaitGroup, ready to be driven via
// Update. RunController/RunStore are left nil: the smoke test never executes the
// returned tea.Cmds (which is the only place those are touched), it only checks that
// producing them, and rendering, never panics.
func newTestModel(t *testing.T) (Model, *sync.WaitGroup) {
	t.Helper()
	var wg sync.WaitGroup
	m := New(Config{ID: "smoke"}, &wg)
	return m, &wg
}

// drive feeds each message through Update in order, asserting no panic, and returns
// the final model so callers can inspect View().
func drive(t *testing.T, m Model, msgs ...tea.Msg) Model {
	t.Helper()
	require.NotPanics(t, func() {
		for _, msg := range msgs {
			next, _ := m.Update(msg)
			m = next.(Model)
			// render on every step: a bad state often only panics when it's drawn.
			_ = m.View()
		}
	})
	return m
}

// synthRun builds a populated *gotest.Run (one passing, one failing, one skipped test
// in a single package) the same way production does: run.Result = *NewResult(...) then
// replay events (see runner.go:74, replay.go:64).
func synthRun() *gotest.Run {
	run := gotest.NewRun(gotest.RunnerConfig{})
	run.Result = *gotest.NewResult(gotest.ResultConfig{TrackOtherOutput: true, TrackFailingOutput: true})

	const pkg = "github.com/example/foo"
	el := 0.01
	emit := func(fn string, conclusion gotest.Action) {
		ref := gotest.Reference{Package: pkg, FuncName: fn}
		run.Result.Update(gotest.Event{Action: gotest.RunAction, Reference: ref})
		run.Result.Update(gotest.Event{Action: conclusion, Elapsed: &el, Reference: ref})
	}
	emit("TestPassing", gotest.PassAction)
	emit("TestFailing", gotest.FailAction)
	emit("TestSkipped", gotest.SkipAction)
	return run
}

var windowSize = tea.WindowSizeMsg{Width: 120, Height: 40}

// TestStudio_EmptySession drives every interactive key against a studio that never
// received a run (a session with zero runs). every status-toggle and nav key funnels
// through code that used to deref a nil RunViewer; this asserts none of them panic and
// that the double-quit path doesn't underflow the WaitGroup.
func TestStudio_EmptySession(t *testing.T) {
	m, _ := newTestModel(t)

	m = drive(t, m,
		windowSize,
		keyMsg("j"), keyMsg("k"), // list nav on an empty list
		keyMsg(" "),                           // toggle multiselect on nothing
		keyMsg("f"), keyMsg("p"), keyMsg("s"), // status filters with no run loaded
		keyMsg("?"),              // toggle help
		keyMsg("a"), keyMsg("r"), // re-run keys with nothing to run
		keyMsg("q"), keyMsg("q"), // double quit must not panic the WaitGroup
	)

	assert.NotEmpty(t, ansi.Strip(m.View()), "empty-session studio should still render its chrome")
}

// TestStudio_PopulatedRun loads a run with a known failing test, then drives navigation
// and filtering. it asserts the failing test surfaces in the rendered output and that
// none of the interactions panic.
func TestStudio_PopulatedRun(t *testing.T) {
	m, _ := newTestModel(t)

	m = drive(t, m,
		windowSize,
		uievent.SwitchTestRun{TestRun: synthRun()},
	)

	view := ansi.Strip(m.View())
	assert.Contains(t, view, "TestFailing", "the failing test should be visible in the references pane")
	assert.True(t, strings.Contains(view, "✘") || strings.Contains(view, "1"),
		"stats line should reflect the failure; got:\n%s", view)

	// now exercise the interactive surface on a real run.
	m = drive(t, m,
		keyMsg("j"), keyMsg("j"), keyMsg("k"), // move the cursor around the list
		keyMsg("f"),              // hide failing tests
		keyMsg("f"),              // show them again
		keyMsg("p"), keyMsg("s"), // toggle passed/skipped visibility
		keyMsg(" "),              // multiselect the current reference
		keyMsg("?"),              // toggle help
		windowSize,               // a resize mid-session
		keyMsg("a"),              // re-run all (cmd is produced, not executed)
		keyMsg("q"), keyMsg("q"), // quit, twice
	)

	assert.NotEmpty(t, ansi.Strip(m.View()))
}
