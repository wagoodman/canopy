package state

import (
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

type DefinitionViewer interface {
	References() []gotest.Reference
}

func NewDefinitionViewer(defs gotest.Definitions) DefinitionViewer {
	return defs
}
