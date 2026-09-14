package gostd

import (
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// writeForeignBuildFailure writes the compiler diagnostic for a package that failed to build because a
// *different* package did (a dependency that does not compile). Go records the diagnostic against the
// package it actually compiled, so without this the only explanation the reader gets is "[build failed]".
// A package broken by its own sources needs nothing here: that diagnostic already belongs to it.
func writeForeignBuildFailure(result *gotest.Result, pkgRef gotest.Reference, write func(gotest.Event)) {
	conclusion := result.ReferenceConclusion(pkgRef)
	if conclusion == nil || conclusion.FailedBuild == "" || conclusion.FailedBuild == pkgRef.Package {
		return
	}

	for _, e := range result.ReferenceEvents(gotest.NewReference(conclusion.FailedBuild, "")) {
		if e.Action != gotest.BuildOutputAction || strings.TrimSpace(e.Output) == "" {
			continue
		}
		write(e)
	}
}
