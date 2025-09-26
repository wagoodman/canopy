package gotest

import (
	"sort"

	"github.com/lindell/go-ordered-set/orderedset"
)

// MinimalSelection returns a minimal set of references that can be used to run tests relative to the provided definitions.
// say for instance, we're given definitions:
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_Handle
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_OnGoTestEvent
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_String
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestNewMultiPackageHandler
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietHandler
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietPackage
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestVerboseHandler
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestVerbosePackage
//
// and given references:
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_Handle
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_OnGoTestEvent
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_String
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestNewMultiPackageHandler
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietHandler
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietPackage
//
// Then the minimal set of references that can be used to run tests would be packages when all test function references
// are provided, and test functions when only some test function references are provided within the greater package:
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietHandler
//   - github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietPackage
//
// References that have t.Run cases should be ignored entirely, however, if there is no function reference for a package,
// then the function reference with no t.Run cases should be used or created.
func MinimalSelection(defs Definitions, refs References) References {
	packageFunctions := buildPackageFunctions(defs)

	packages, selectedFunctions, selectedPackages := collectPackagesAndSelections(refs)

	var result References

	// process each package
	for pkg := range packages {
		if shouldUsePackageReference(pkg, packageFunctions, selectedFunctions, selectedPackages) {
			result = append(result, Reference{Package: pkg})
		} else {
			// only some functions are selected, add individual function references
			if selectedFuncs, exists := selectedFunctions[pkg]; exists {
				for funcName := range selectedFuncs {
					result = append(result, Reference{
						Package:  pkg,
						FuncName: funcName,
					})
				}
			}
		}
	}

	if len(result) == 0 {
		return nil
	}

	// sort results for deterministic output
	sort.Sort(result)

	return result
}

// buildPackageFunctions creates a map of package -> set of all test functions defined in that package
func buildPackageFunctions(defs Definitions) map[string]map[string]bool {
	packageFunctions := make(map[string]map[string]bool)
	for _, def := range defs {
		if packageFunctions[def.ImportPath] == nil {
			packageFunctions[def.ImportPath] = make(map[string]bool)
		}
		packageFunctions[def.ImportPath][def.FnName] = true
	}
	return packageFunctions
}

// collectPackagesAndSelections processes references to collect packages and their selected functions
func collectPackagesAndSelections(refs References) (map[string]bool, map[string]map[string]bool, map[string]bool) {
	packages := make(map[string]bool)
	selectedFunctions := make(map[string]map[string]bool)
	selectedPackages := make(map[string]bool)

	for _, ref := range refs {
		packages[ref.Package] = true

		// if it's a package reference, mark the entire package as selected
		if ref.IsPackage() {
			selectedPackages[ref.Package] = true
			continue
		}

		// if it's a function reference (not a subtest), track it
		if ref.FuncName != "" && ref.TRunName == "" {
			if selectedFunctions[ref.Package] == nil {
				selectedFunctions[ref.Package] = make(map[string]bool)
			}
			selectedFunctions[ref.Package][ref.FuncName] = true
		}
	}

	return packages, selectedFunctions, selectedPackages
}

// shouldUsePackageReference determines if a package should be referenced as a whole package
// rather than individual function references
func shouldUsePackageReference(pkg string, packageFunctions map[string]map[string]bool, selectedFunctions map[string]map[string]bool, selectedPackages map[string]bool) bool {
	// if the entire package was explicitly selected AND no individual functions are selected, add package reference
	if selectedPackages[pkg] && (selectedFunctions[pkg] == nil || len(selectedFunctions[pkg]) == 0) {
		return true
	}

	// check if all defined functions are selected AND no undefined functions are selected
	allDefinedFunctionsSelected := true
	hasUndefinedFunctions := false

	if definedFuncs, exists := packageFunctions[pkg]; exists && len(definedFuncs) > 0 {
		// check if all defined functions are selected
		for funcName := range definedFuncs {
			if selectedFunctions[pkg] == nil || !selectedFunctions[pkg][funcName] {
				allDefinedFunctionsSelected = false
				break
			}
		}

		// check if any selected functions are not defined
		if selectedFunctions[pkg] != nil {
			for funcName := range selectedFunctions[pkg] {
				if !definedFuncs[funcName] {
					hasUndefinedFunctions = true
					break
				}
			}
		}
	} else {
		allDefinedFunctionsSelected = false
	}

	return allDefinedFunctionsSelected && !hasUndefinedFunctions && len(packageFunctions[pkg]) > 0
}

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
