package options

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wagoodman/canopy/cmd/canopy/internal/env"
)

func TestShardIndex_EnvFallback(t *testing.T) {
	t.Setenv(ShardEnv, "2/4")

	o := ShardIndex{}
	require.NoError(t, o.PostLoad())
	i, n, src, err := o.Resolve(env.NewSnapshotEnvironmentGetter(nil))
	require.NoError(t, err)
	assert.Equal(t, []any{2, 4, "CANOPY_TEST_SHARD (env)"}, []any{i, n, src})

	// the flag wins
	o = ShardIndex{Value: "1/3"}
	require.NoError(t, o.PostLoad())
	_, _, src, err = o.Resolve(env.NewSnapshotEnvironmentGetter(nil))
	require.NoError(t, err)
	assert.Equal(t, "flag", src)

	// commands without --shard ignore it
	o = ShardIndex{Disabled: true}
	require.NoError(t, o.PostLoad())
	assert.False(t, o.Enabled())

	// auto reads the CI pair
	t.Setenv(ShardEnv, ShardAuto)
	o = ShardIndex{}
	require.NoError(t, o.PostLoad())
	_, _, src, err = o.Resolve(env.NewSnapshotEnvironmentGetter(map[string]string{"CI_NODE_INDEX": "1", "CI_NODE_TOTAL": "2"}))
	require.NoError(t, err)
	assert.Equal(t, "CI_NODE_INDEX/CI_NODE_TOTAL", src)

	t.Setenv(ShardEnv, "5/4")
	o = ShardIndex{}
	require.Error(t, o.PostLoad())
}
