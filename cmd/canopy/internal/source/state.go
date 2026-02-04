package source

import (
	"os"
	"path/filepath"
	"time"

	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
)

// State captures the git repository state at a point in time.
type State struct {
	Commit     string
	Branch     string
	Dirty      bool
	DirtyFiles []FileState
}

// FileState represents a single dirty .go file's state.
type FileState struct {
	Path        string
	ContentHash string     // xxhash64 hex, empty if file deleted/unreadable
	ModTime     *time.Time // nil if file deleted
}

// CaptureState captures the current git state for the given directory.
// Returns nil if the directory is not within a git repository.
func CaptureState(dir string) *State {
	repo, err := openRepo(dir)
	if err != nil {
		log.WithFields("error", err).Warn("failed to open git repo for source state")
		return nil
	}
	if repo == nil {
		return nil
	}

	info, err := getGitInfo(repo)
	if err != nil {
		log.WithFields("error", err).Warn("failed to get git info")
		return nil
	}

	dirtyPaths, err := getDirtyGoPaths(repo)
	if err != nil {
		log.WithFields("error", err).Warn("failed to get dirty go paths")
		return nil
	}

	// resolve the repo root for building absolute paths
	wt, err := repo.Worktree()
	if err != nil {
		log.WithFields("error", err).Warn("failed to get worktree")
		return nil
	}
	repoRoot := wt.Filesystem.Root()

	var files []FileState
	for _, p := range dirtyPaths {
		absPath := filepath.Join(repoRoot, p)
		fs := FileState{Path: p}

		stat, err := os.Stat(absPath)
		if err != nil {
			// file was deleted or otherwise inaccessible
			files = append(files, fs)
			continue
		}

		mt := stat.ModTime()
		fs.ModTime = &mt

		hash, err := hashFile(absPath)
		if err != nil {
			log.WithFields("path", p, "error", err).Debug("failed to hash dirty file")
			files = append(files, fs)
			continue
		}
		fs.ContentHash = hash

		files = append(files, fs)
	}

	return &State{
		Commit:     info.Commit,
		Branch:     info.Branch,
		Dirty:      len(files) > 0,
		DirtyFiles: files,
	}
}
