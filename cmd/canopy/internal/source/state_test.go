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

func TestCaptureState(t *testing.T) {
	t.Run("not a git repo", func(t *testing.T) {
		dir := t.TempDir()
		state := CaptureState(dir)
		require.Nil(t, state)
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
		hash, err := wt.Commit("initial", &git.CommitOptions{
			Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
		})
		require.NoError(t, err)

		state := CaptureState(dir)
		require.NotNil(t, state)
		require.Equal(t, hash.String(), state.Commit)
		require.Equal(t, "master", state.Branch)
		require.False(t, state.Dirty)
		require.Empty(t, state.DirtyFiles)
	})

	t.Run("dirty repo with modified and untracked go files", func(t *testing.T) {
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

		// modify tracked file
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n// changed\n"), 0o644))

		// add untracked go file
		require.NoError(t, os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main\n"), 0o644))

		// add non-go file (should be excluded)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# hi\n"), 0o644))

		state := CaptureState(dir)
		require.NotNil(t, state)
		require.True(t, state.Dirty)
		require.Len(t, state.DirtyFiles, 2)

		// files should be sorted
		require.Equal(t, "main.go", state.DirtyFiles[0].Path)
		require.NotEmpty(t, state.DirtyFiles[0].ContentHash)
		require.NotNil(t, state.DirtyFiles[0].ModTime)

		require.Equal(t, "new.go", state.DirtyFiles[1].Path)
		require.NotEmpty(t, state.DirtyFiles[1].ContentHash)
		require.NotNil(t, state.DirtyFiles[1].ModTime)
	})

	t.Run("deleted go file has empty hash and nil modtime", func(t *testing.T) {
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

		require.NoError(t, os.Remove(filepath.Join(dir, "main.go")))

		state := CaptureState(dir)
		require.NotNil(t, state)
		require.True(t, state.Dirty)
		require.Len(t, state.DirtyFiles, 1)
		require.Equal(t, "main.go", state.DirtyFiles[0].Path)
		require.Empty(t, state.DirtyFiles[0].ContentHash)
		require.Nil(t, state.DirtyFiles[0].ModTime)
	})

	t.Run("repo with no commits", func(t *testing.T) {
		dir := t.TempDir()
		_, err := git.PlainInit(dir, false)
		require.NoError(t, err)

		// add an untracked go file
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644))

		state := CaptureState(dir)
		require.NotNil(t, state)
		require.Equal(t, "", state.Commit)
		require.Equal(t, "", state.Branch)
		require.True(t, state.Dirty)
		require.Len(t, state.DirtyFiles, 1)
	})

	t.Run("works from subdirectory", func(t *testing.T) {
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

		sub := filepath.Join(dir, "pkg")
		require.NoError(t, os.MkdirAll(sub, 0o755))

		state := CaptureState(sub)
		require.NotNil(t, state)
		require.False(t, state.Dirty)
	})
}
