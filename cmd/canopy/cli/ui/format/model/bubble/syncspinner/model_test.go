package syncspinner

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"
)

func TestGlassFrames(t *testing.T) {
	// fills to a full cell, then drains back to an empty (but still one column wide) cell
	require.Equal(t, []string{"⡀", "⡄", "⣄", "⣆", "⣦", "⣧", "⣷", "⣿", "⢿", "⢻", "⠻", "⠹", "⠙", "⠘", "⠈", "⠀"}, Glass.Frames)

	for _, f := range Glass.Frames {
		// anything wider shifts the status column and breaks row alignment
		require.Equal(t, 1, lipgloss.Width(f), "frame %q", f)
	}
}
