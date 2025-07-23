package gotest

import "sort"

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
	
	// create a map of package -> set of all test functions defined in that package
	packageFunctions := make(map[string]map[string]bool)
	for _, def := range defs {
		if packageFunctions[def.ImportPath] == nil {
			packageFunctions[def.ImportPath] = make(map[string]bool)
		}
		packageFunctions[def.ImportPath][def.FnName] = true
	}

	// collect all packages that have references
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

	var result References
	
	// process each package
	for pkg := range packages {
		// if the entire package was explicitly selected AND no individual functions are selected, add package reference
		if selectedPackages[pkg] && (selectedFunctions[pkg] == nil || len(selectedFunctions[pkg]) == 0) {
			result = append(result, Reference{Package: pkg})
			continue
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
		
		if allDefinedFunctionsSelected && !hasUndefinedFunctions && len(packageFunctions[pkg]) > 0 {
			// all functions in package are selected, use package reference
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
