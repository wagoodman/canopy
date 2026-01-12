package cienv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectWith_GitHubActions(t *testing.T) {
	env := DetectWith(func(key string) string {
		if key == "GITHUB_ACTIONS" {
			return "true"
		}
		return ""
	})

	require.NotNil(t, env)
	assert.Equal(t, "GitHub Actions", env.Name)
	assert.True(t, env.SupportsGrouping)
}

func TestDetectWith_AzurePipelines(t *testing.T) {
	env := DetectWith(func(key string) string {
		if key == "TF_BUILD" {
			return "True"
		}
		return ""
	})

	require.NotNil(t, env)
	assert.Equal(t, "Azure Pipelines", env.Name)
	assert.True(t, env.SupportsGrouping)
}

func TestDetectWith_NoCI(t *testing.T) {
	env := DetectWith(func(key string) string {
		return ""
	})

	assert.Nil(t, env)
}

func TestIsCIWith(t *testing.T) {
	tests := []struct {
		name   string
		getenv func(string) string
		want   bool
	}{
		{
			name: "GitHub Actions",
			getenv: func(key string) string {
				if key == "GITHUB_ACTIONS" {
					return "true"
				}
				return ""
			},
			want: true,
		},
		{
			name: "not CI",
			getenv: func(key string) string {
				return ""
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCIWith(tt.getenv)
			assert.Equal(t, tt.want, got)
		})
	}
}
