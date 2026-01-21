package ide

import (
	"fmt"
	"os/exec"

	"github.com/wagoodman/canopy/cmd/canopy/internal/env"
)

// Zed provides integration with the Zed editor.
type Zed struct {
	// binPath is the path to the zed command-line binary.
	binPath string
}

// NewZed creates a new Zed Context. If lookPathFunc is nil, os/exec.LookPath
// is used to locate the zed binary. Returns an error if the binary is not found.
func NewZed(lookPathFunc func(string) (string, error)) (*Zed, error) {
	if lookPathFunc == nil {
		lookPathFunc = exec.LookPath
	}
	path, err := lookPathFunc("zed")
	if err != nil {
		return nil, fmt.Errorf("unable to find zed binary: %w", err)
	}
	return &Zed{
		binPath: path,
	}, nil
}

// isActive checks if Zed is the active editor by examining environment variables
// set by Zed's terminal.
func (z Zed) isActive(e env.EnvironmentGetter) bool {
	if e.Getenv("__CFBundleIdentifier") == "dev.zed.Zed" {
		return true
	}
	if e.Getenv("ZED_TERM") == "true" {
		return true
	}
	return false
}

// OpenFileAtLineCommand returns the shell command to open a file at a specific
// line in Zed using the file:line:column format.
func (z Zed) OpenFileAtLineCommand(filePath string, line int) string {
	return fmt.Sprintf(`%s "%s:%d:0"`, z.binPath, filePath, line)
}

// FileAtLineURL returns a file:// URL for the given file and line.
func (z Zed) FileAtLineURL(file string, line int) string {
	return fileAtLineURL(file, line)
}
