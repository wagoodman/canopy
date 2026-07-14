package source

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
)

// writeCommit writes content to a file under the repo dir, stages it, and commits, returning the
// commit hash. Used by the git plumbing tests to build small fixture histories.
func writeCommit(t *testing.T, repo *git.Repository, dir, name, content, msg string) plumbing.Hash {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add(name)
	require.NoError(t, err)
	hash, err := wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
	})
	require.NoError(t, err)
	return hash
}

// checkoutBranch switches to (optionally creating) a branch.
func checkoutBranch(t *testing.T, repo *git.Repository, name string, create bool) {
	t.Helper()
	wt, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(name),
		Create: create,
	}))
}

func TestDefaultBranch(t *testing.T) {
	t.Run("not a repo", func(t *testing.T) {
		_, err := DefaultBranch(t.TempDir())
		require.Error(t, err)
	})

	t.Run("master fallback", func(t *testing.T) {
		dir := t.TempDir()
		repo, err := git.PlainInit(dir, false)
		require.NoError(t, err)
		writeCommit(t, repo, dir, "main.go", "package main\n", "initial")

		branch, err := DefaultBranch(dir)
		require.NoError(t, err)
		require.Equal(t, "master", branch)
	})

	t.Run("main preferred over master", func(t *testing.T) {
		dir := t.TempDir()
		repo, err := git.PlainInit(dir, false)
		require.NoError(t, err)
		writeCommit(t, repo, dir, "main.go", "package main\n", "initial")

		// create a "main" branch alongside the default "master"; probe order prefers main
		checkoutBranch(t, repo, "main", true)

		branch, err := DefaultBranch(dir)
		require.NoError(t, err)
		require.Equal(t, "main", branch)
	})
}

func TestMergeBase(t *testing.T) {
	t.Run("not a repo", func(t *testing.T) {
		_, err := MergeBase(t.TempDir(), "master")
		require.Error(t, err)
	})

	t.Run("common ancestor for diverged branch", func(t *testing.T) {
		dir := t.TempDir()
		repo, err := git.PlainInit(dir, false)
		require.NoError(t, err)

		// A on master, then feature diverges at B while master advances to C
		base := writeCommit(t, repo, dir, "main.go", "package main\n", "A")
		checkoutBranch(t, repo, "feature", true)
		writeCommit(t, repo, dir, "feature.go", "package main\n", "B")
		checkoutBranch(t, repo, "master", false)
		writeCommit(t, repo, dir, "main.go", "package main\n// C\n", "C")
		checkoutBranch(t, repo, "feature", false)

		got, err := MergeBase(dir, "master")
		require.NoError(t, err)
		require.Equal(t, base.String(), got)
	})
}

// note: the "@auto" working-tree-vs-branch decision now lives in the verify command
// (resolveTargetFiles) so it can report the diff basis; see the commands package tests.

func TestOpenRepo(t *testing.T) {
	t.Run("valid repo", func(t *testing.T) {
		dir := t.TempDir()
		_, err := git.PlainInit(dir, false)
		require.NoError(t, err)

		repo, err := openRepo(dir)
		require.NoError(t, err)
		require.NotNil(t, repo)
	})

	t.Run("not a repo", func(t *testing.T) {
		dir := t.TempDir()

		repo, err := openRepo(dir)
		require.NoError(t, err)
		require.Nil(t, repo)
	})

	t.Run("detects repo from subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		_, err := git.PlainInit(dir, false)
		require.NoError(t, err)

		sub := filepath.Join(dir, "sub", "dir")
		require.NoError(t, os.MkdirAll(sub, 0o755))

		repo, err := openRepo(sub)
		require.NoError(t, err)
		require.NotNil(t, repo)
	})
}

func TestGetGitInfo(t *testing.T) {
	t.Run("repo with commit on branch", func(t *testing.T) {
		dir := t.TempDir()
		repo, err := git.PlainInit(dir, false)
		require.NoError(t, err)

		// create a file and commit
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644))
		wt, err := repo.Worktree()
		require.NoError(t, err)
		_, err = wt.Add("main.go")
		require.NoError(t, err)
		hash, err := wt.Commit("initial commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "test",
				Email: "test@test.com",
				When:  time.Now(),
			},
		})
		require.NoError(t, err)

		info, err := getGitInfo(repo)
		require.NoError(t, err)
		require.Equal(t, hash.String(), info.Commit)
		require.Equal(t, "master", info.Branch)
	})

	t.Run("repo with no commits", func(t *testing.T) {
		dir := t.TempDir()
		repo, err := git.PlainInit(dir, false)
		require.NoError(t, err)

		info, err := getGitInfo(repo)
		require.NoError(t, err)
		require.Equal(t, "", info.Commit)
		require.Equal(t, "", info.Branch)
	})
}

func TestGetDirtyGoPaths(t *testing.T) {
	t.Run("modified and untracked go files", func(t *testing.T) {
		dir := t.TempDir()
		repo, err := git.PlainInit(dir, false)
		require.NoError(t, err)

		// create and commit a file
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644))
		wt, err := repo.Worktree()
		require.NoError(t, err)
		_, err = wt.Add("main.go")
		require.NoError(t, err)
		_, err = wt.Commit("initial", &git.CommitOptions{
			Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
		})
		require.NoError(t, err)

		// modify the tracked file
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n// modified\n"), 0o644))

		// add an untracked go file
		require.NoError(t, os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main\n"), 0o644))

		// add a non-go file (should be excluded)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# hi\n"), 0o644))

		paths, err := getDirtyGoPaths(repo)
		require.NoError(t, err)
		require.Equal(t, []string{"main.go", "new.go"}, paths)
	})

	t.Run("clean repo", func(t *testing.T) {
		dir := t.TempDir()
		repo, err := git.PlainInit(dir, false)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644))
		wt, err := repo.Worktree()
		require.NoError(t, err)
		_, err = wt.Add("main.go")
		require.NoError(t, err)
		_, err = wt.Commit("initial", &git.CommitOptions{
			Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
		})
		require.NoError(t, err)

		paths, err := getDirtyGoPaths(repo)
		require.NoError(t, err)
		require.Empty(t, paths)
	})

	t.Run("deleted go file shows up", func(t *testing.T) {
		dir := t.TempDir()
		repo, err := git.PlainInit(dir, false)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644))
		wt, err := repo.Worktree()
		require.NoError(t, err)
		_, err = wt.Add("main.go")
		require.NoError(t, err)
		_, err = wt.Commit("initial", &git.CommitOptions{
			Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
		})
		require.NoError(t, err)

		// delete the file
		require.NoError(t, os.Remove(filepath.Join(dir, "main.go")))

		paths, err := getDirtyGoPaths(repo)
		require.NoError(t, err)
		require.Equal(t, []string{"main.go"}, paths)
	})
}
