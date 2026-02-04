package source

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
)

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
