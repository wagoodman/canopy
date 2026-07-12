package options

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFlakyPostLoad_WindowDaySuffix(t *testing.T) {
	// the documented "7d" window must be accepted (time.ParseDuration alone rejects it)
	o := DefaultFlaky()
	o.WindowStr = "7d"

	require.NoError(t, o.PostLoad())
	require.Equal(t, 7*24*time.Hour, o.Window)
}
