package ide

import (
	"fmt"
	"os/exec"
)

// VSCode provides integration with Microsoft Visual Studio Code.
type VSCode struct {
	// binPath is the path to the code command-line binary.
	binPath string
}

// NewVSCode creates a new VSCode Context. If lookPathFunc is nil, os/exec.LookPath
// is used to locate the code binary. Returns an error if the binary is not found.
func NewVSCode(lookPathFunc func(string) (string, error)) (*VSCode, error) {
	if lookPathFunc == nil {
		lookPathFunc = exec.LookPath
	}
	path, err := lookPathFunc("code")
	if err != nil {
		return nil, fmt.Errorf("unable to find code binary: %w", err)
	}
	return &VSCode{
		binPath: path,
	}, nil
}

// isActive checks if VS Code is the active IDE by examining the TERM_PROGRAM
// environment variable.
func (v VSCode) isActive(env EnvironmentGetter) bool {
	return env.Getenv("TERM_PROGRAM") == "vscode"
}

// OpenFileAtLineCommand returns the shell command to open a file at a specific
// line in VS Code using the --goto flag.
func (v VSCode) OpenFileAtLineCommand(filePath string, line int) string {
	return fmt.Sprintf(`%s --goto "%s:%d"`, v.binPath, filePath, line)
}

// FileAtLineURL returns a file:// URL for the given file and line.
func (v VSCode) FileAtLineURL(file string, line int) string {
	return fileAtLineURL(file, line)
}
