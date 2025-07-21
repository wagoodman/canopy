package toggle

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

var (
	engagedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#D16DFF"))
	disengagedStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#909090",
		Dark:  "#626262",
	})
)

type Toggle struct {
	key.Binding

	engaged        bool
	engagedDesc    string
	disengagedDesc string

	EngagedStyle    *lipgloss.Style
	DisengagedStyle *lipgloss.Style
}

type Option func(*Toggle)

type Toggles []Toggle

func (t Toggles) Keys() []key.Binding {
	keys := make([]key.Binding, len(t))
	for i, toggle := range t {
		keys[i] = toggle.Binding
	}
	return keys
}

func New(keyBinding key.Binding, opts ...Option) Toggle {
	t := Toggle{
		Binding: keyBinding,
	}

	for _, opt := range opts {
		opt(&t)
	}

	defaultDesc := t.Binding.Help().Desc

	if defaultDesc != "" {
		if t.engagedDesc == "" {
			t.engagedDesc = t.Binding.Help().Desc
		}

		if t.disengagedDesc == "" {
			t.disengagedDesc = t.Binding.Help().Desc
		}
	}

	if t.EngagedStyle == nil {
		t.EngagedStyle = &engagedStyle
	}

	if t.DisengagedStyle == nil {
		t.DisengagedStyle = &disengagedStyle
	}

	t.setHelp()
	return t
}

func WithDescription(desc string) Option {
	return func(t *Toggle) {
		t.engagedDesc = desc
		t.disengagedDesc = desc
	}
}

func WithEngagedDescription(desc string) Option {
	return func(t *Toggle) {
		t.engagedDesc = desc
	}
}

func WithDisengagedDescription(desc string) Option {
	return func(t *Toggle) {
		t.disengagedDesc = desc
	}
}

func WithEngagedStyle(style *lipgloss.Style) Option {
	return func(t *Toggle) {
		t.EngagedStyle = style
	}
}

func WithDisengagedStyle(style *lipgloss.Style) Option {
	return func(t *Toggle) {
		t.DisengagedStyle = style
	}
}

func WithEngaged(engaged bool) Option {
	return func(t *Toggle) {
		t.engaged = engaged
	}
}

func WithStyle(engagedStyle, disengagedStyle *lipgloss.Style) Option {
	return func(t *Toggle) {
		t.EngagedStyle = engagedStyle
		t.DisengagedStyle = disengagedStyle
	}
}

func (i Toggle) Engaged() bool {
	return i.engaged
}

func (i *Toggle) Press() {

	i.engaged = !i.engaged
	i.setHelp()
}

func (i *Toggle) setHelp() {
	k := i.Binding.Help().Key
	if k == "" {
		keys := i.Keys()
		if len(keys) > 0 {
			k = keys[0]
		}
	}
	if i.engaged {
		if i.engagedDesc != "" {
			i.SetHelp(k, i.engagedDesc)
		}
	} else {
		if i.disengagedDesc != "" {
			i.SetHelp(k, i.disengagedDesc)
		}
	}
}
