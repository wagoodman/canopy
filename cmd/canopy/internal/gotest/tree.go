package gotest

import (
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
)

// Tree organizes test references into a hierarchical structure for navigation and selection.
// It maintains parent-child relationships between packages, functions, and subtests.
type Tree struct {
	Nodes map[Reference]*TreeNode
	Roots *OrderedReferenceSet
}

// TreeNode represents a single node in the test hierarchy tree, containing its reference,
// associated events, and child nodes.
type TreeNode struct {
	Reference Reference
	Events    []Event
	Children  *OrderedReferenceSet
}

// OrderedReferenceSet maintains an ordered collection of unique references, preserving
// insertion order while preventing duplicates.
type OrderedReferenceSet struct {
	ordered []Reference
	refs    mapset.Set[Reference]
}

// NewTree creates an empty test hierarchy tree.
func NewTree() *Tree {
	return &Tree{
		Nodes: make(map[Reference]*TreeNode),
		Roots: newOrderedReferenceSet(),
	}
}

// NewTreeFromDefinitions creates a tree populated with references from test definitions.
// Automatically builds the hierarchical structure from the definition data.
func NewTreeFromDefinitions(defs []Definition) *Tree {
	t := NewTree()
	for _, def := range defs {
		t.Add(NewReferencesFromDefinition(def)...)
	}
	return t
}

// NewTreeFromReferences creates a tree populated with the given references.
// Builds parent-child relationships automatically based on reference hierarchy.
func NewTreeFromReferences(refs []Reference) *Tree {
	t := NewTree()
	t.Add(refs...)
	return t
}

func newTreeNodeFromEvent(e Event) *TreeNode {
	return &TreeNode{
		Children:  newOrderedReferenceSet(),
		Events:    []Event{e},
		Reference: e.Reference,
	}
}

func newTreeNodeFromReference(ref Reference) *TreeNode {
	return &TreeNode{
		Children:  newOrderedReferenceSet(),
		Reference: ref,
	}
}

func newOrderedReferenceSet(refs ...Reference) *OrderedReferenceSet {
	s := &OrderedReferenceSet{
		refs: mapset.NewSet[Reference](),
	}

	s.PushBack(refs...)

	return s
}

func (o *OrderedReferenceSet) PushFront(refs ...Reference) {
	for _, ref := range refs {
		if o.refs.Add(ref) {
			o.ordered = append([]Reference{ref}, o.ordered...)
		}
	}
}

func (o *OrderedReferenceSet) PushBack(refs ...Reference) {
	for _, ref := range refs {
		if o.refs.Add(ref) {
			o.ordered = append(o.ordered, ref)
		}
	}
}

func (o *OrderedReferenceSet) Ordered() []Reference {
	return o.ordered
}

func (o *OrderedReferenceSet) Reversed() []Reference {
	rev := make([]Reference, len(o.ordered))
	for i := 0; i < len(o.ordered); i++ {
		rev[i] = o.ordered[len(o.ordered)-i-1]
	}
	return rev
}

func (o *OrderedReferenceSet) Contains(ref Reference) bool {
	return o.refs.Contains(ref)
}

func (o *OrderedReferenceSet) Copy() *OrderedReferenceSet {
	return &OrderedReferenceSet{
		ordered: append([]Reference{}, o.ordered...),
		refs:    o.refs.Clone(),
	}
}

func (o *OrderedReferenceSet) Len() int {
	return o.refs.Cardinality()
}

func (o *OrderedReferenceSet) PopFront() *Reference {
	if o.Len() == 0 {
		return nil
	}

	ref := o.ordered[0]
	o.ordered = o.ordered[1:]
	o.refs.Remove(ref)
	return &Reference{
		Package:  ref.Package,
		FuncName: ref.FuncName,
		TRunName: ref.TRunName,
	}
}

func (o *OrderedReferenceSet) PopBack() *Reference {
	if o.Len() == 0 {
		return nil
	}

	ref := o.ordered[len(o.ordered)-1]
	o.ordered = o.ordered[:len(o.ordered)-1]
	o.refs.Remove(ref)
	return &Reference{
		Package:  ref.Package,
		FuncName: ref.FuncName,
		TRunName: ref.TRunName,
	}
}

func (t *Tree) Add(refs ...Reference) {
	for _, ref := range refs {
		_, exists := t.Nodes[ref]
		if exists {
			continue // skip this ref, not the rest of the batch
		}

		t.addNode(newTreeNodeFromReference(ref))
	}
}

func (t *Tree) Update(e Event) {
	node, exists := t.Nodes[e.Reference]
	if !exists {
		t.addNode(newTreeNodeFromEvent(e))
		return
	}

	node.Events = append(node.Events, e)
}

func (t *Tree) addNode(n *TreeNode) {
	t.ensureAncestorBranch(n.Reference)
	t.Nodes[n.Reference] = n
	parentRef := n.Reference.ParentRef()
	if parentRef != nil {
		t.addChildToNode(*parentRef, n.Reference)
	} else {
		t.Roots.PushBack(n.Reference)
	}
}

func (t *Tree) ensureAncestorBranch(ref Reference) {
	last := ref
	ancestor := ref.ParentRef()

	// add all the parents up to the root
	for {
		if ancestor == nil {
			t.Roots.PushBack(last)
			break
		}
		if _, exists := t.Nodes[*ancestor]; exists {
			break
		}
		newAncestorNode := newTreeNodeFromReference(*ancestor)
		t.Nodes[*ancestor] = newAncestorNode

		// prep next iteration...
		last = *ancestor
		ancestor = ancestor.ParentRef()
	}
}

func (t *Tree) addChildToNode(parent, ref Reference) {
	n, exists := t.Nodes[parent]
	if !exists {
		t.ensureAncestorBranch(parent)
	}
	n.AddChild(ref)
	t.Nodes[parent] = n
}

func (n *TreeNode) AddChild(r Reference) {
	if n.Children.Contains(r) {
		return
	}
	n.Children.PushBack(r)
}

func (n TreeNode) Output() string {
	sb := strings.Builder{}
	for _, e := range n.Events {
		sb.WriteString(e.Output)
	}
	return sb.String()
}

func (n TreeNode) LatestEvent() *Event {
	if len(n.Events) == 0 {
		return nil
	}
	e := n.Events[len(n.Events)-1].Copy()
	return &e
}

type TreeIterator interface {
	Next() *TreeNode
}

type dfsTreeIterator struct {
	tree     *Tree
	refQueue *OrderedReferenceSet
}

func (d *dfsTreeIterator) Next() *TreeNode {
	nextRef := d.refQueue.PopFront()
	if nextRef == nil {
		return nil
	}
	node := d.tree.Nodes[*nextRef]

	d.refQueue.PushFront(node.Children.Reversed()...)

	return node
}

type bfsTreeIterator struct {
	tree     *Tree
	refQueue *OrderedReferenceSet
}

func (d *bfsTreeIterator) Next() *TreeNode {
	nextRef := d.refQueue.PopFront()
	if nextRef == nil {
		return nil
	}
	node := d.tree.Nodes[*nextRef]

	d.refQueue.PushBack(node.Children.Ordered()...)

	return node
}

func (t *Tree) IterateDF(start ...Reference) TreeIterator {
	if len(start) == 0 {
		// crawl the whole tree
		start = make([]Reference, t.Roots.Len())
		copy(start, t.Roots.Ordered())
	}

	return &dfsTreeIterator{
		refQueue: newOrderedReferenceSet(start...),
		tree:     t,
	}
}

func (t *Tree) IterateBF(start ...Reference) TreeIterator {
	if len(start) == 0 {
		// crawl the whole tree
		start = make([]Reference, t.Roots.Len())
		copy(start, t.Roots.Ordered())
	}

	return &bfsTreeIterator{
		refQueue: newOrderedReferenceSet(start...),
		tree:     t,
	}
}
