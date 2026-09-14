package state

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/syncspinner"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
)

func TestCommon_OnMessage_Canceled(t *testing.T) {
	spin := syncspinner.New()
	c := Common{Spinner: spin.CurrentTick()}

	c.OnMessage(tea.KeyMsg{Type: tea.KeyCtrlC})
	require.True(t, c.Canceled)
	require.Equal(t, style.CanceledGlyph, c.Spinner.View)

	// a late tick must not bring the spinner back
	tick := spin.CurrentTick()
	tick.View = "⠼"
	c.OnMessage(tick)
	require.Equal(t, style.CanceledGlyph, c.Spinner.View)
}
