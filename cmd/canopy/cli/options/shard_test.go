package options

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wagoodman/canopy/cmd/canopy/internal/env"
)

func TestParseShard(t *testing.T) {
	tests := []struct {
		in      string
		i, n    int
		wantErr bool
	}{
		{in: "1/4", i: 1, n: 4},
		{in: "4/4", i: 4, n: 4},
		{in: "1/1", i: 1, n: 1},
		{in: "0/4", wantErr: true},
		{in: "5/4", wantErr: true},
		{in: "1/0", wantErr: true},
		{in: "-1/4", wantErr: true},
		{in: "1", wantErr: true},
		{in: "a/b", wantErr: true},
		{in: "1/4/2", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			i, n, err := ParseShard(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.i, i)
			assert.Equal(t, tt.n, n)
		})
	}
}

func TestShardIndex_Resolve(t *testing.T) {
	flag := ShardIndex{Value: "2/3"}
	require.NoError(t, flag.PostLoad())
	i, n, src, err := flag.Resolve(env.NewSnapshotEnvironmentGetter(nil))
	require.NoError(t, err)
	assert.Equal(t, []any{2, 3, "flag"}, []any{i, n, src})

	auto := ShardIndex{Value: ShardAuto}
	require.NoError(t, auto.PostLoad())
	i, n, src, err = auto.Resolve(env.NewSnapshotEnvironmentGetter(map[string]string{"CI_NODE_INDEX": "2", "CI_NODE_TOTAL": "4"}))
	require.NoError(t, err)
	assert.Equal(t, []any{2, 4, "CI_NODE_INDEX/CI_NODE_TOTAL"}, []any{i, n, src})

	// no parallelism falls back to 1/1
	i, n, src, err = auto.Resolve(env.NewSnapshotEnvironmentGetter(nil))
	require.NoError(t, err)
	assert.Equal(t, []any{1, 1, ""}, []any{i, n, src})

	_, _, _, err = auto.Resolve(env.NewSnapshotEnvironmentGetter(map[string]string{"CI_NODE_INDEX": "2"}))
	require.Error(t, err)
}
