package xflagset

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

// TestPrintSectionsWithLocal verifies the catch-all rendering: local flags not routed into a named
// group still appear (under a generic "Flags:" heading), grouped flags are not duplicated into it,
// and the auto-added help flag is omitted.
func TestPrintSectionsWithLocal(t *testing.T) {
	nfs := NewNamed()
	grouped := nfs.FlagSet("State")
	grouped.String("store-dir", "", "where to store")

	local := pflag.NewFlagSet("cmd", pflag.ContinueOnError)
	local.String("store-dir", "", "where to store") // also in the State group
	local.String("output", "", "output format")     // ungrouped
	local.Bool("help", false, "help")               // must be skipped

	out := nfs.printSectionsWithLocal(local, -1)

	require.Contains(t, out, "State flags:")
	require.Contains(t, out, "Flags:")
	require.Contains(t, out, "--output")
	require.NotContains(t, out, "--help")
	// store-dir belongs to a group; it must not also appear in the ungrouped catch-all
	require.Equal(t, 1, strings.Count(out, "--store-dir"))
}
