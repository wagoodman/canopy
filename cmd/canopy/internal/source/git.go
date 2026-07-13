package source

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/scylladb/go-set/strset"
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

// worktreeRoot returns the absolute filesystem root of the repository's worktree.
func worktreeRoot(repo *git.Repository) (string, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	return wt.Filesystem.Root(), nil
}

// ChangedGoFiles returns absolute paths of dirty .go files (staged, unstaged, and
// untracked) in the git repository containing dir. Returns an error if dir is not
// within a git repository.
func ChangedGoFiles(dir string) ([]string, error) {
	repo, err := openRepo(dir)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, fmt.Errorf("%s is not within a git repository", dir)
	}

	rel, err := getDirtyGoPaths(repo)
	if err != nil {
		return nil, err
	}

	root, err := worktreeRoot(repo)
	if err != nil {
		return nil, err
	}
	return toAbs(root, rel), nil
}

// ChangedGoFilesSince returns absolute paths of .go files that differ between the given
// git ref and the current working tree (committed changes ref..HEAD plus any dirty files).
// This mirrors the semantics of `git diff --name-only <ref>`.
func ChangedGoFilesSince(dir, ref string) ([]string, error) {
	repo, err := openRepo(dir)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, fmt.Errorf("%s is not within a git repository", dir)
	}

	committed, err := changedGoPathsSince(repo, ref)
	if err != nil {
		return nil, err
	}

	dirty, err := getDirtyGoPaths(repo)
	if err != nil {
		return nil, err
	}

	// union of committed diff and working-tree changes, so nothing possibly-affected is missed
	set := strset.New(committed...)
	set.Add(dirty...)
	rel := set.List()
	sort.Strings(rel)

	root, err := worktreeRoot(repo)
	if err != nil {
		return nil, err
	}
	return toAbs(root, rel), nil
}

// changedGoPathsSince returns sorted repo-relative .go paths that changed between the
// given ref's tree and HEAD's tree.
func changedGoPathsSince(repo *git.Repository, ref string) ([]string, error) {
	refHash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, fmt.Errorf("unable to resolve ref %q: %w", ref, err)
	}
	refCommit, err := repo.CommitObject(*refHash)
	if err != nil {
		return nil, err
	}
	refTree, err := refCommit.Tree()
	if err != nil {
		return nil, err
	}

	head, err := repo.Head()
	if err != nil {
		return nil, err
	}
	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, err
	}
	headTree, err := headCommit.Tree()
	if err != nil {
		return nil, err
	}

	changes, err := refTree.Diff(headTree)
	if err != nil {
		return nil, err
	}

	set := strset.New()
	for _, c := range changes {
		// a change can rename/delete, so capture both endpoints
		for _, name := range []string{c.From.Name, c.To.Name} {
			if strings.HasSuffix(name, ".go") {
				set.Add(name)
			}
		}
	}
	paths := set.List()
	sort.Strings(paths)
	return paths, nil
}

// toAbs joins each repo-relative path to root.
func toAbs(root string, rel []string) []string {
	abs := make([]string, len(rel))
	for i, p := range rel {
		abs[i] = filepath.Join(root, p)
	}
	return abs
}
