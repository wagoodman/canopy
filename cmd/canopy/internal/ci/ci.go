// Package ci provides detection for CI environments.
package ci

import (
	"fmt"
	"strconv"

	"github.com/wagoodman/canopy/cmd/canopy/internal/env"
)

// Provider identifies the CI system type.
type Provider string

const (
	// ProviderUnknown indicates no known CI detected.
	ProviderUnknown Provider = ""
	// ProviderGitHub indicates GitHub Actions.
	ProviderGitHub Provider = "github"
	// ProviderAzure indicates Azure Pipelines.
	ProviderAzure Provider = "azure"
	// ProviderGitLab indicates GitLab CI.
	ProviderGitLab Provider = "gitlab"
)

// Detect returns the detected CI provider, or ProviderUnknown if not in a known CI.
func Detect() Provider {
	return DetectWith(&env.OSEnvironmentGetter{})
}

// DetectWith returns the detected CI provider using a custom environment getter.
func DetectWith(e env.EnvironmentGetter) Provider {
	// GitHub Actions detection
	// https://docs.github.com/en/actions/learn-github-actions/environment-variables#default-environment-variables
	if env.Truthy(e.Getenv("GITHUB_ACTIONS")) {
		return ProviderGitHub
	}

	// Azure Pipelines detection
	// https://learn.microsoft.com/en-us/azure/devops/pipelines/build/variables
	if env.Truthy(e.Getenv("TF_BUILD")) {
		return ProviderAzure
	}

	// GitLab CI detection
	// https://docs.gitlab.com/ee/ci/variables/predefined_variables.html
	if env.Truthy(e.Getenv("GITLAB_CI")) {
		return ProviderGitLab
	}

	return ProviderUnknown
}

// shardVars describes a CI provider's parallelism variable pair.
type shardVars struct {
	index, total string
	base         int // the provider's first index (0 or 1)
}

var shardProviders = []shardVars{
	{"CI_NODE_INDEX", "CI_NODE_TOTAL", 1},                         // gitlab
	{"CIRCLE_NODE_INDEX", "CIRCLE_NODE_TOTAL", 0},                 // circleci
	{"BUILDKITE_PARALLEL_JOB", "BUILDKITE_PARALLEL_JOB_COUNT", 0}, // buildkite
	{"SYSTEM_JOBPOSITIONINPHASE", "SYSTEM_TOTALJOBSINPHASE", 1},   // azure
}

// ShardFromEnv resolves a 1-based shard index and total from the CI provider's own variables. The
// source is the "INDEX_VAR/TOTAL_VAR" pair used, or empty (with 1/1) when none is set. Providers
// are keyed on the index var: GitLab sets only the total (as 1) on jobs that are not parallel.
func ShardFromEnv(e env.EnvironmentGetter) (index, total int, source string, err error) {
	for _, p := range shardProviders {
		rawIndex := e.Getenv(p.index)
		if rawIndex == "" {
			continue
		}
		source = p.index + "/" + p.total

		i, err := strconv.Atoi(rawIndex)
		if err != nil {
			return 0, 0, source, fmt.Errorf("%s: invalid value %q", p.index, rawIndex)
		}
		rawTotal := e.Getenv(p.total)
		if rawTotal == "" {
			return 0, 0, source, fmt.Errorf("%s is set but %s is not", p.index, p.total)
		}
		n, err := strconv.Atoi(rawTotal)
		if err != nil {
			return 0, 0, source, fmt.Errorf("%s: invalid value %q", p.total, rawTotal)
		}

		i += 1 - p.base
		if n < 1 || i < 1 || i > n {
			return 0, 0, source, fmt.Errorf("%s/%s: shard %d of %d is out of range", p.index, p.total, i, n)
		}
		return i, n, source, nil
	}
	return 1, 1, "", nil
}
