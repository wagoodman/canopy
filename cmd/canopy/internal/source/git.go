package source

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/scylladb/go-set/strset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
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

// DefaultBranch resolves the repository's default branch name (e.g. "main" or "master").
// It first consults the remote's advertised default via refs/remotes/origin/HEAD, then falls
// back to probing for local "main" and "master" branches. Returns an error if none resolve,
// leaving the fallback decision to the caller.
func DefaultBranch(dir string) (string, error) {
	repo, err := openRepo(dir)
	if err != nil {
		return "", err
	}
	if repo == nil {
		return "", fmt.Errorf("%s is not within a git repository", dir)
	}

	// prefer the remote's advertised default, e.g. refs/remotes/origin/HEAD -> refs/remotes/origin/main
	if ref, err := repo.Reference(plumbing.NewRemoteHEADReferenceName("origin"), false); err == nil {
		if target := ref.Target(); target.IsRemote() {
			// short form is "origin/main"; drop the remote name to get the branch name
			if short := strings.TrimPrefix(target.Short(), "origin/"); short != "" {
				return short, nil
			}
		}
	}

	// fall back to probing common local default branch names
	for _, name := range []string{"main", "master"} {
		if _, err := repo.Reference(plumbing.NewBranchReferenceName(name), false); err == nil {
			return name, nil
		}
	}

	return "", fmt.Errorf("unable to determine default branch for %s", dir)
}

// MergeBase returns the merge-base commit SHA between baseRef and HEAD. Errors if either side
// cannot be resolved or the histories share no common ancestor.
func MergeBase(dir, baseRef string) (string, error) {
	repo, err := openRepo(dir)
	if err != nil {
		return "", err
	}
	if repo == nil {
		return "", fmt.Errorf("%s is not within a git repository", dir)
	}

	baseHash, err := repo.ResolveRevision(plumbing.Revision(baseRef))
	if err != nil {
		return "", fmt.Errorf("unable to resolve ref %q: %w", baseRef, err)
	}
	baseCommit, err := repo.CommitObject(*baseHash)
	if err != nil {
		return "", err
	}

	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return "", err
	}

	bases, err := baseCommit.MergeBase(headCommit)
	if err != nil {
		return "", err
	}
	if len(bases) == 0 {
		return "", fmt.Errorf("no common ancestor between %q and HEAD", baseRef)
	}
	return bases[0].Hash.String(), nil
}

// FilesChangedInCommit returns the repo-relative .go paths changed by the given commit,
// diffing it against its first parent (against the empty tree for a root commit). This names
// the likely culprit files for a blame annotation. Reuses the same go-git tree diff the
// since-ref path uses rather than shelling out.
func FilesChangedInCommit(dir, commitSHA string) ([]string, error) {
	log.WithFields("commit", commitSHA).Trace("resolving files changed in commit")
	repo, err := openRepo(dir)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, fmt.Errorf("%s is not within a git repository", dir)
	}

	commit, err := repo.CommitObject(plumbing.NewHash(commitSHA))
	if err != nil {
		return nil, fmt.Errorf("unable to load commit %q: %w", commitSHA, err)
	}
	commitTree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	// a root commit has no parent, so diff against an empty tree (every file is "added").
	var parentTree *object.Tree
	if commit.NumParents() > 0 {
		parent, err := commit.Parent(0)
		if err != nil {
			return nil, err
		}
		parentTree, err = parent.Tree()
		if err != nil {
			return nil, err
		}
	}

	changes, err := parentTree.Diff(commitTree)
	if err != nil {
		return nil, err
	}

	set := strset.New()
	for _, c := range changes {
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

// CommitsBetween returns the number of commits strictly between the good and bad commits
// following the first-parent chain from bad (0 when bad is the immediate child of good).
// Returns -1 when good is not an ancestor of bad within the walk cap, so the caller can treat
// the distance as unknown (and report a range) rather than fabricating adjacency.
//
// ponytail: first-parent walk with a cap, not a full ancestry graph. merge commits off the
// first-parent line are not traversed, so a bad reachable from good only through a side branch
// reads as unknown (-1). that is the honest, conservative answer for confidence; upgrade to a
// full rev-list walk if merge-heavy histories need tighter exact/range calls.
func CommitsBetween(dir, goodSHA, badSHA string) (int, error) {
	log.WithFields("good", goodSHA, "bad", badSHA).Trace("counting commits between good and bad")
	repo, err := openRepo(dir)
	if err != nil {
		return -1, err
	}
	if repo == nil {
		return -1, fmt.Errorf("%s is not within a git repository", dir)
	}

	good := plumbing.NewHash(goodSHA)
	if good == plumbing.NewHash(badSHA) {
		return 0, nil
	}

	commit, err := repo.CommitObject(plumbing.NewHash(badSHA))
	if err != nil {
		return -1, fmt.Errorf("unable to load commit %q: %w", badSHA, err)
	}

	const maxSteps = 5000
	for steps := 0; steps < maxSteps; steps++ {
		if commit.NumParents() == 0 {
			return -1, nil // reached a root without finding good
		}
		parent, err := commit.Parent(0)
		if err != nil {
			return -1, err
		}
		if parent.Hash == good {
			return steps, nil
		}
		commit = parent
	}
	return -1, nil
}

// toAbs joins each repo-relative path to root.
func toAbs(root string, rel []string) []string {
	abs := make([]string, len(rel))
	for i, p := range rel {
		abs[i] = filepath.Join(root, p)
	}
	return abs
}
