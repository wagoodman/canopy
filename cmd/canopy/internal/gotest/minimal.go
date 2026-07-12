package gotest

import (
	"github.com/lindell/go-ordered-set/orderedset"
)

// GroupIntoRuns takes a set of references and makes the fewest number of groups possible in order to run tests.
// So for instance, say we're given:
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/a
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/b
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/c
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_Handle
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_OnGoTestEvent
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_String
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestNewMultiPackageHandler
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietHandler
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietPackage
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestVerboseHandler
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestVerbosePackage
//
// We need to make the fewest number of groups possible to run tests. So any packages with specific test functions
// we assume that these functions do not represent the full set of tests in the package, and therefore we cannot
// run the entire package. Instead we result to `-run FUNC` for the specific functions. Any packages that do not
// have specific test functions, we assume that the package is complete and can be run as a whole (thus we can
// group all such packages together). By the end of this function, we would have the following groups:
//
// group 1:
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/a
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/b
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/c
//
// group 2:
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_Handle
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_OnGoTestEvent
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_String
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestNewMultiPackageHandler
//
// group 3:
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietHandler
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietPackage
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestVerboseHandler
// - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestVerbosePackage
//
// Note: for the most effective grouping ensure to call `MinimalSelection` first to ensure that the references are minimal.
func GroupIntoRuns(refs References) []References {
	var grouped []References
	if len(refs) == 0 {
		return grouped
	}

	var pkgOnlyGroup References
	pkgGroups := make(map[string]References)

	orderedPkgs := orderedset.New[string]()
	for _, ref := range refs {
		orderedPkgs.Add(ref.Package)

		if ref.IsPackage() {
			// if it's a package reference, add it to the package-only group
			pkgOnlyGroup = append(pkgOnlyGroup, ref)
			continue
		}

		// if it's a function reference, add it to the specific package group
		pkgGroups[ref.Package] = append(pkgGroups[ref.Package], ref)
	}

	var finalGroups []References
	if len(pkgOnlyGroup) > 0 {
		// if we have a package-only group, add it as the first group
		finalGroups = append(finalGroups, pkgOnlyGroup)
	}

	// we want a consistent order of packages, so we iterate through the ordered set
	for _, pkg := range orderedPkgs.Values() {
		if group, exists := pkgGroups[pkg]; exists && len(group) > 0 {
			// if we have a specific package group, add it to the final groups
			finalGroups = append(finalGroups, group)
		}
	}

	return finalGroups
}
