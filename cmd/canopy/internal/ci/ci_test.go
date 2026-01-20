package ci

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestDetectWith_NoCI(t *testing.T) {
	e := env.NewSnapshotEnvironmentGetter(map[string]string{})

	result := DetectWith(e)

	assert.Equal(t, ProviderUnknown, result)
}
