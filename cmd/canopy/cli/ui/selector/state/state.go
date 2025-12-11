package state

import (
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// DefinitionViewer provides read-only access to test definitions.
// it allows filtering references by custom predicates.
type DefinitionViewer interface {
	// References returns all test references, optionally filtered by the given predicates.
	// each predicate should return true to remove a reference from the result.
	References(removeFilters ...func(gotest.Reference) bool) []gotest.Reference
}

// NewDefinitionViewer creates a DefinitionViewer from the given test definitions.
// the returned viewer uses the Definitions type's built-in References method.
func NewDefinitionViewer(defs gotest.Definitions) DefinitionViewer {
	return defs
}
