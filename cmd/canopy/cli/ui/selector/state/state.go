package state

import (
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

type DefinitionViewer interface {
	References(removeFilters ...func(gotest.Reference) bool) []gotest.Reference
}

func NewDefinitionViewer(defs gotest.Definitions) DefinitionViewer {
	return defs
}
