package ide

import (
	"fmt"
	"os/exec"

	"github.com/wagoodman/canopy/cmd/canopy/internal/env"
)

var _ Context = (*Goland)(nil)

// Goland provides integration with JetBrains GoLand IDE.
type Goland struct {
	// binPath is the path to the goland command-line binary.
	binPath string
}

// NewGoland creates a new Goland Context. If lookPathFunc is nil, os/exec.LookPath
// is used to locate the goland binary. Returns an error if the binary is not found.
func NewGoland(lookPathFunc func(string) (string, error)) (*Goland, error) {
	if lookPathFunc == nil {
		lookPathFunc = exec.LookPath
	}
	path, err := lookPathFunc("goland")
	if err != nil {
		return nil, fmt.Errorf("unable to find goland binary: %w", err)
	}
	return &Goland{
		binPath: path,
	}, nil
}

// isActive checks if GoLand is the active IDE by examining environment variables
// set by GoLand's terminal.
func (g Goland) isActive(e env.EnvironmentGetter) bool {
	if e.Getenv("__CFBundleIdentifier") == "com.jetbrains.goland" {
		return true
	}

	if e.Getenv("TERMINAL_EMULATOR") == "JetBrains-JediTerm" {
		return true
	}
	return false
}

// OpenFileAtLineCommand returns the shell command to open a file at a specific
// line in GoLand.
func (g Goland) OpenFileAtLineCommand(filePath string, line int) string {
	return fmt.Sprintf("%s --line %d %s", g.binPath, line, filePath)
}

// FileAtLineURL returns a file:// URL for the given file and line.
func (g Goland) FileAtLineURL(file string, line int) string {
	return fileAtLineURL(file, line)
}
