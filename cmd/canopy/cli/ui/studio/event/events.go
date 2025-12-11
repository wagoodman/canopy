// Package event defines Bubble Tea event types used for communication between
// studio UI components.
package event

import "github.com/wagoodman/canopy/cmd/canopy/internal/gotest"

// FilteringInput indicates that a pane has entered or exited filtering mode.
// When filtering is active, alphanumeric input is routed only to that pane.
type FilteringInput struct {
	// Name is the identifier of the pane that is filtering.
	Name string

	// Completed is true when filtering has finished; false when it starts.
	Completed bool
}

// SwitchTestRun signals that the UI should switch to viewing a different test run.
type SwitchTestRun struct {
	// TestRun is the new test run to display.
	TestRun *gotest.Run
}

// SelectedTestReferences indicates which test references the user has selected
// in the references pane.
type SelectedTestReferences struct {
	// All is true if all tests are selected.
	All bool

	// Refs contains the selected test references (packages, functions, or test cases).
	Refs []gotest.Reference
}

// ActionError represents an error that occurred during a user action.
type ActionError struct {
	// Message describes the error that occurred.
	Message string
}
