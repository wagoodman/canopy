// Package ci provides detection for CI environments.
package ci

import (
	"strings"

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
	if truthy(e.Getenv("GITHUB_ACTIONS")) {
		return ProviderGitHub
	}

	// Azure Pipelines detection
	// https://learn.microsoft.com/en-us/azure/devops/pipelines/build/variables
	if truthy(e.Getenv("TF_BUILD")) {
		return ProviderAzure
	}

	// GitLab CI detection
	// https://docs.gitlab.com/ee/ci/variables/predefined_variables.html
	if truthy(e.Getenv("GITLAB_CI")) {
		return ProviderGitLab
	}

	return ProviderUnknown
}

func truthy(input string) bool {
	switch strings.ToLower(input) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
