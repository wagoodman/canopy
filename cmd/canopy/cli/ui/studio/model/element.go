package model

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/xhelp"
)

// Element represents a UI component that can be rendered, updated, and provides
// keybindings. It combines Bubble Tea's Model interface with the help system.
type Element interface {
	tea.Model
	xhelp.KeyMap
}

// Sizer provides read access to a component's dimensions.
type Sizer interface {
	// Width returns the component's current width in characters.
	Width() int

	// Height returns the component's current height in lines.
	Height() int
}

// Shaper provides write access to a component's dimensions, allowing the
// layout system to control sizing.
type Shaper interface {
	// SetWidth sets the component's width in characters.
	SetWidth(int)

	// SetHeight sets the component's height in lines.
	SetHeight(int)
}
