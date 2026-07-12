package referencespane

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSelectedItem_EmptyList guards against the nil type-assertion panic that
// happened when a run had zero function references (empty visible list).
func TestSelectedItem_EmptyList(t *testing.T) {
	d := newListItemDelegate()
	l := list.New([]list.Item{}, d, 0, 0)

	_, _, ok := d.selectedItem(&l)
	assert.False(t, ok, "expected ok=false for an empty list")

	// neither of these should panic on an empty list
	require.NotPanics(t, func() { d.onNavigate(&l) })
	require.NotPanics(t, func() { d.onToggleMultiselect(&l) })
	require.NotPanics(t, func() { d.selectedTestReferencesCmd(&l) })
}
