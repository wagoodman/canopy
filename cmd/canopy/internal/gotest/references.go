package gotest

import "github.com/lindell/go-ordered-set/orderedset"

// References is a collection of test references with sorting and utility methods.
type References []Reference

// Len implements sort.Interface for References.
func (r References) Len() int {
	return len(r)
}

// Less implements sort.Interface for References, sorting by package, then function, then subtest.
func (r References) Less(i, j int) bool {
	if r[i].Package == r[j].Package {
		if r[i].FuncName == r[j].FuncName {
			return r[i].TRunName < r[j].TRunName
		}
		return r[i].FuncName < r[j].FuncName
	}
	return r[i].Package < r[j].Package
}

// Swap implements sort.Interface for References.
func (r References) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

// TestFunctionsCount returns the number of function-level test references (excluding subtests and packages).
func (r References) TestFunctionsCount() int {
	count := 0
	for _, ref := range r {
		if ref.FuncName != "" && ref.TRunName == "" {
			count++
		}
	}
	return count
}

// Packages returns a deduplicated, ordered list of all package paths referenced in the collection.
func (r References) Packages() []string {
	pkgs := orderedset.New[string]()
	for _, ref := range r {
		if ref.Package != "" {
			pkgs.Add(ref.Package)
		}
	}

	return pkgs.Values()
}

// NewReferencesFromDefinition converts a test definition into a collection of executable references.
// Creates references for the function itself plus all discovered t.Run subtests.
func NewReferencesFromDefinition(def Definition) []Reference {
	fnName := def.FnName
	pkgName := def.ImportPath
	refs := []Reference{
		{
			Package:  pkgName,
			FuncName: fnName,
		},
	}

	caseNames := rewriteTestNames(def.Cases...)

	for _, testCase := range caseNames {
		refs = append(refs, Reference{
			Package:  pkgName,
			FuncName: fnName,
			TRunName: testCase,
		})
	}

	return refs
}

type node struct {
	ref      Reference
	children map[string]*node
	isLeaf   bool
}

func newNode(ref Reference) *node {
	return &node{
		ref:      ref,
		children: make(map[string]*node),
		isLeaf:   false,
	}
}

// MinimizeReferences creates a selection tree from selected references, prunes it based on the
// total tree, and returns the minimized set of references. This operation is useful for reducing
// a large selection down to its essential components - for example, if all children of a node
// are selected, the selection can be reduced to just the parent node.
func MinimizeReferences(total, selected []Reference) []Reference {
	totalTree := buildTree(total)
	selectionTree := buildTree(selected)

	pruneTree(selectionTree, totalTree)

	return collectMinimizedReferences(selectionTree)
}

func buildTree(refs []Reference) *node {
	root := newNode(Reference{})

	for _, ref := range refs {
		current := root

		if _, ok := current.children[ref.Package]; !ok {
			current.children[ref.Package] = newNode(Reference{Package: ref.Package})
		}
		current = current.children[ref.Package]

		if ref.FuncName != "" {
			if _, ok := current.children[ref.FuncName]; !ok {
				current.children[ref.FuncName] = newNode(Reference{Package: ref.Package, FuncName: ref.FuncName})
			}
			current = current.children[ref.FuncName]
		}

		if ref.TRunName != "" {
			if _, ok := current.children[ref.TRunName]; !ok {
				current.children[ref.TRunName] = newNode(ref)
				current.children[ref.TRunName].isLeaf = true
			}
		}
	}

	return root
}

func pruneTree(selNode, totNode *node) bool {
	if selNode.isLeaf {
		// if the selection node is a leaf, it's a complete test case selected
		return true
	}

	allChildrenSelected := true
	for key, selChild := range selNode.children {
		totChild, ok := totNode.children[key]
		if !ok {
			// if the total tree does not have this child, don't prune
			allChildrenSelected = false
			continue
		}
		if !pruneTree(selChild, totChild) {
			// if the recursive call returns false, not all children are selected
			allChildrenSelected = false
		}
	}

	if allChildrenSelected && len(selNode.children) == len(totNode.children) {
		// if all children are selected and match the total tree, prune them
		selNode.children = make(map[string]*node) // remove all children
		selNode.isLeaf = true                     // mark as leaf because all children are pruned
		return true
	}

	return false
}

func collectMinimizedReferences(node *node) []Reference {
	var result []Reference

	if len(node.children) == 0 {
		if (node.ref.Package != "") || (node.ref.FuncName != "") || (node.ref.TRunName != "") {
			result = append(result, node.ref)
		}
		return result
	}

	for _, child := range node.children {
		result = append(result, collectMinimizedReferences(child)...)
	}

	return result
}
