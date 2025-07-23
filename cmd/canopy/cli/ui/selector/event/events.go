package event

import "github.com/wagoodman/canopy/cmd/canopy/internal/gotest"

type SwitchState struct {
	Definitions gotest.Definitions
}

type SelectedTestReferences struct {
	//All  bool
	Refs []gotest.Reference
}

type RefreshReferences struct {
	AboutToFilter bool
}

type SetFiltering struct {
	Enabled bool
}
