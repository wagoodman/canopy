package event

import "github.com/wagoodman/canopy/cmd/canopy/internal/gotest"

type FilteringInput struct {
	Name      string
	Completed bool
}

type SwitchTestRun struct {
	TestRun *gotest.Run
}

type SelectedTestReferences struct {
	All  bool
	Refs []gotest.Reference
}

type ActionError struct {
	Message string
}
