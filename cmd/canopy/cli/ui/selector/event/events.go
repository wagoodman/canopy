package event

import "github.com/wagoodman/canopy/cmd/canopy/internal/gotest"

type SwitchState struct {
	Definitions gotest.Definitions
	Selected    gotest.References
}

type SelectedTestReferences struct {
	Finished bool
	Refs     []gotest.Reference
}

type RefreshReferences struct {
	AboutToFilter bool
}
