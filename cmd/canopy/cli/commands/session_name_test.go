package commands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveSessionName(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string // exact expected value; empty means "assert non-empty" instead
	}{
		{name: "literal passes through", value: "debug-flaky", want: "debug-flaky"},
		{name: "unknown @-resolver falls back", value: "@bogus", want: "default"},
		// the repo is a git repo with a module, so these all resolve to something concrete
		{name: "default resolves to branch", value: ""},
		{name: "@branch resolves", value: "@branch"},
		{name: "@module resolves", value: "@module"},
		{name: "@worktree resolves", value: "@worktree"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveSessionName(tt.value)
			if tt.want != "" {
				require.Equal(t, tt.want, got)
				return
			}
			require.NotEmpty(t, got)
		})
	}
}
