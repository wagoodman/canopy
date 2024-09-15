package model

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/xhelp"
)

type Element interface {
	tea.Model
	xhelp.KeyMap
}

type Sizer interface {
	Width() int
	Height() int
}

type Shaper interface {
	SetWidth(int)
	SetHeight(int)
}
