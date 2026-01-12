// Package cienv provides detection and utilities for CI environments.
package cienv

import (
	"os"
)

// CIType identifies the CI system type.
type CIType int

const (
	// CITypeUnknown indicates no known CI detected.
	CITypeUnknown CIType = iota
	// CITypeGitHub indicates GitHub Actions.
	CITypeGitHub
	// CITypeAzure indicates Azure Pipelines.
	CITypeAzure
	// CITypeGitLab indicates GitLab CI.
	CITypeGitLab
)

// Environment represents a CI environment with its capabilities.
type Environment struct {
	// Name is the human-readable name of the CI environment.
	Name string
	// Type identifies the CI system for syntax selection.
	Type CIType
	// SupportsGrouping indicates whether the CI supports collapsible output groups.
	SupportsGrouping bool
}

// Detect returns the detected CI environment, or nil if not in a known CI.
func Detect() *Environment {
	return DetectWith(os.Getenv)
}

// DetectWith returns the detected CI environment using a custom environment getter.
func DetectWith(getenv func(string) string) *Environment {
	// GitHub Actions detection
	// https://docs.github.com/en/actions/learn-github-actions/environment-variables#default-environment-variables
	if getenv("GITHUB_ACTIONS") == "true" {
		return &Environment{
			Name:             "GitHub Actions",
			Type:             CITypeGitHub,
			SupportsGrouping: true,
		}
	}

	// Azure Pipelines detection
	// https://learn.microsoft.com/en-us/azure/devops/pipelines/build/variables
	if getenv("TF_BUILD") == "True" {
		return &Environment{
			Name:             "Azure Pipelines",
			Type:             CITypeAzure,
			SupportsGrouping: true,
		}
	}

	// GitLab CI detection
	// https://docs.gitlab.com/ee/ci/variables/predefined_variables.html
	if getenv("GITLAB_CI") == "true" {
		return &Environment{
			Name:             "GitLab CI",
			Type:             CITypeGitLab,
			SupportsGrouping: true,
		}
	}

	return nil
}

// IsCI returns true if running in any detected CI environment.
func IsCI() bool {
	return Detect() != nil
}

// IsCIWith returns true if running in any detected CI environment, using a custom getter.
func IsCIWith(getenv func(string) string) bool {
	return DetectWith(getenv) != nil
}
