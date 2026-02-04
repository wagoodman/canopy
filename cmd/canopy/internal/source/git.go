package source

import (
	"errors"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type gitInfo struct {
	Commit string // HEAD commit hash (full SHA), empty if no commits
	Branch string // branch name, "HEAD" if detached, empty if no commits
}

// openRepo attempts to open a git repo at the given path (walks up to find .git).
// Returns nil, nil if not a git repo.
func openRepo(dir string) (*git.Repository, error) {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			return nil, nil
		}
		return nil, err
	}
	return repo, nil
}

// getGitInfo returns the HEAD commit hash and branch name from the repository.
// Returns empty strings if the repository has no commits.
func getGitInfo(repo *git.Repository) (*gitInfo, error) {
	ref, err := repo.Head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			// no commits yet
			return &gitInfo{}, nil
		}
		return nil, err
	}

	info := &gitInfo{
		Commit: ref.Hash().String(),
	}

	if ref.Name().IsBranch() {
		info.Branch = ref.Name().Short()
	} else {
		info.Branch = "HEAD"
	}

	return info, nil
}

// getDirtyGoPaths returns sorted paths of dirty .go files relative to the repo root.
// This includes modified tracked files and untracked non-ignored files.
func getDirtyGoPaths(repo *git.Repository) ([]string, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	status, err := wt.Status()
	if err != nil {
		return nil, err
	}

	var paths []string
	for path, fileStatus := range status {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		if fileStatus.Staging == git.Unmodified && fileStatus.Worktree == git.Unmodified {
			continue
		}
		paths = append(paths, path)
	}

	sort.Strings(paths)
	return paths, nil
}
