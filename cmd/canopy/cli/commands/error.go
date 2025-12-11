package commands

// SilentError represents an error that can optionally suppress its output when returned from a command.
// This is useful for distinguishing between errors that should be displayed to the user and those
// that should exit silently (e.g., test failures that are already shown in the UI).
type SilentError interface {
	error
	// IsSilent returns true if the error message should not be displayed to the user.
	IsSilent() bool
}
