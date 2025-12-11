package ide

import "fmt"

// Context represents an IDE integration that can open files at specific line numbers
// and detect if it's the currently active IDE.
type Context interface {
	// isActive checks if this IDE is currently active based on environment variables.
	isActive(env EnvironmentGetter) bool

	// OpenFileAtLineCommand returns the shell command to open a file at a specific line.
	OpenFileAtLineCommand(file string, line int) string

	// FileAtLineURL returns a file:// URL for the given file and line number.
	FileAtLineURL(file string, line int) string
}

// fileAtLineURL constructs a standard file:// URL with line number.
func fileAtLineURL(file string, line int) string {
	return fmt.Sprintf("file://%s:%d", file, line)
}
