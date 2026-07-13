package commands

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/source"
)

// sessionNameFallback is used whenever an "@"-resolver can't produce a concrete name.
const sessionNameFallback = "default"

// defaultSessionName is the default --session value: follow the current git branch.
const defaultSessionName = "@branch"

// resolveSessionName maps a raw --session flag value to a concrete session name. Values without a
// leading "@" are literal names used as-is. "@branch" (also the default for "") resolves the current
// git branch, "@module" the go module path, "@worktree" the worktree root basename. Anything
// unresolvable (unknown "@xxx", not in a repo, detached HEAD, errors) falls back to "default".
func resolveSessionName(value string) string {
	switch value {
	case "", defaultSessionName:
		return resolveBranch()
	case "@module":
		return resolveModule()
	case "@worktree":
		return resolveWorktree()
	}
	// literal names pass through; unrecognized "@xxx" falls back to default
	if strings.HasPrefix(value, "@") {
		return sessionNameFallback
	}
	return value
}

// resolveBranch returns the current git branch, or "default" outside a repo / on detached HEAD.
func resolveBranch() string {
	state := source.CaptureState(".")
	if state == nil || state.Branch == "" || state.Branch == "HEAD" {
		return sessionNameFallback
	}
	return state.Branch
}

// resolveModule returns the current go module path via go list, or "default" on error.
func resolveModule() string {
	pkgs, err := golist.SelectPackages([]string{"."}, nil)
	if err != nil || pkgs == nil || pkgs.Module() == "" {
		return sessionNameFallback
	}
	return pkgs.Module()
}

// resolveWorktree returns the basename of the git worktree root, or "default" on error.
func resolveWorktree() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return sessionNameFallback
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return sessionNameFallback
	}
	return filepath.Base(root)
}
