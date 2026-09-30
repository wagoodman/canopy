package ci

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/env"
)

func TestDetectWith_GitHubActions(t *testing.T) {
	e := env.NewSnapshotEnvironmentGetter(map[string]string{
		"GITHUB_ACTIONS": "true",
	})

	result := DetectWith(e)

	assert.Equal(t, ProviderGitHub, result)
}

func TestDetectWith_AzurePipelines(t *testing.T) {
	e := env.NewSnapshotEnvironmentGetter(map[string]string{
		"TF_BUILD": "True",
	})

	result := DetectWith(e)

	assert.Equal(t, ProviderAzure, result)
}

func TestDetectWith_GitLabCI(t *testing.T) {
	e := env.NewSnapshotEnvironmentGetter(map[string]string{
		"GITLAB_CI": "true",
	})

	result := DetectWith(e)

	assert.Equal(t, ProviderGitLab, result)
}

func TestShardFromEnv(t *testing.T) {
	const (
		gitlab    = "CI_NODE_INDEX/CI_NODE_TOTAL"
		circle    = "CIRCLE_NODE_INDEX/CIRCLE_NODE_TOTAL"
		buildkite = "BUILDKITE_PARALLEL_JOB/BUILDKITE_PARALLEL_JOB_COUNT"
		azure     = "SYSTEM_JOBPOSITIONINPHASE/SYSTEM_TOTALJOBSINPHASE"
	)
	tests := []struct {
		name       string
		env        map[string]string
		wantIndex  int
		wantTotal  int
		wantSource string
		wantErr    bool
	}{
		{"no vars", nil, 1, 1, "", false},
		{"gitlab", map[string]string{"CI_NODE_INDEX": "2", "CI_NODE_TOTAL": "4"}, 2, 4, gitlab, false},
		{"gitlab non-parallel job only sets total", map[string]string{"CI_NODE_TOTAL": "1"}, 1, 1, "", false},
		{"circleci is 0-based", map[string]string{"CIRCLE_NODE_INDEX": "0", "CIRCLE_NODE_TOTAL": "4"}, 1, 4, circle, false},
		{"circleci last", map[string]string{"CIRCLE_NODE_INDEX": "3", "CIRCLE_NODE_TOTAL": "4"}, 4, 4, circle, false},
		{"buildkite is 0-based", map[string]string{"BUILDKITE_PARALLEL_JOB": "1", "BUILDKITE_PARALLEL_JOB_COUNT": "3"}, 2, 3, buildkite, false},
		{"azure", map[string]string{"SYSTEM_JOBPOSITIONINPHASE": "3", "SYSTEM_TOTALJOBSINPHASE": "5"}, 3, 5, azure, false},
		{"index without total", map[string]string{"CI_NODE_INDEX": "2"}, 0, 0, gitlab, true},
		{"non-numeric index", map[string]string{"CI_NODE_INDEX": "x", "CI_NODE_TOTAL": "4"}, 0, 0, gitlab, true},
		{"non-numeric total", map[string]string{"CI_NODE_INDEX": "1", "CI_NODE_TOTAL": "four"}, 0, 0, gitlab, true},
		{"gitlab index zero", map[string]string{"CI_NODE_INDEX": "0", "CI_NODE_TOTAL": "4"}, 0, 0, gitlab, true},
		{"gitlab index above total", map[string]string{"CI_NODE_INDEX": "5", "CI_NODE_TOTAL": "4"}, 0, 0, gitlab, true},
		{"circleci index equals total", map[string]string{"CIRCLE_NODE_INDEX": "4", "CIRCLE_NODE_TOTAL": "4"}, 0, 0, circle, true},
		{"zero total", map[string]string{"CI_NODE_INDEX": "1", "CI_NODE_TOTAL": "0"}, 0, 0, gitlab, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i, n, src, err := ShardFromEnv(env.NewSnapshotEnvironmentGetter(tt.env))
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantIndex, i)
			assert.Equal(t, tt.wantTotal, n)
			assert.Equal(t, tt.wantSource, src)
		})
	}
}

func TestDetectWith_NoCI(t *testing.T) {
	e := env.NewSnapshotEnvironmentGetter(map[string]string{})

	result := DetectWith(e)

	assert.Equal(t, ProviderUnknown, result)
}
