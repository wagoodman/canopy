package ide

import "github.com/wagoodman/canopy/cmd/canopy/internal/env"

// Select detects and returns the currently active IDE Context by checking
// environment variables. Returns a dummy context if no supported IDE is detected.
func Select(e env.EnvironmentGetter) Context {
	var available []Context
	if c, err := NewZed(nil); err == nil {
		available = append(available, c)
	}

	if c, err := NewGoland(nil); err == nil {
		available = append(available, c)
	}

	if c, err := NewVSCode(nil); err == nil {
		available = append(available, c)
	}

	for _, c := range available {
		if c.isActive(e) {
			return c
		}
	}

	return &dummy{}
}

// dummy is a no-op Context used when no IDE is detected.
type dummy struct {
}

// isActive always returns true for the dummy context.
func (d dummy) isActive(_ env.EnvironmentGetter) bool {
	return true
}

// OpenFileAtLineCommand returns an empty string as no IDE command is available.
func (d dummy) OpenFileAtLineCommand(_ string, _ int) string {
	return ""
}

// FileAtLineURL returns a standard file:// URL.
func (d dummy) FileAtLineURL(file string, line int) string {
	return fileAtLineURL(file, line)
}
