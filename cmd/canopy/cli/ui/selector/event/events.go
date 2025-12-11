package event

import "github.com/wagoodman/canopy/cmd/canopy/internal/gotest"

// SwitchState signals that new test definitions should be loaded.
// it carries both the definitions to display and any pre-selected references.
type SwitchState struct {
	// Definitions contains the test definitions to display in the selector.
	Definitions gotest.Definitions
	// Selected contains test references that should be pre-selected (e.g., from -run flags).
	Selected gotest.References
}

// SelectedTestReferences carries the user's current test selection.
// it is emitted whenever the selection changes or is confirmed.
type SelectedTestReferences struct {
	// Finished indicates whether the user has confirmed the selection (pressed enter).
	Finished bool
	// Refs contains the selected test references.
	Refs []gotest.Reference
}

// RefreshReferences signals that the reference list should be refreshed.
// it is used to coordinate display format changes when entering or exiting filter mode.
type RefreshReferences struct {
	// AboutToFilter indicates whether the list is about to enter filter mode.
	// when true, references should be displayed in long form for better filtering.
	AboutToFilter bool
}
