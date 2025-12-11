package golist

import (
	"fmt"
	"os"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
)

// SelectPackages discovers packages matching the given paths and filters them using exclude patterns.
// Paths can be local filesystem paths (e.g., "./pkg/...") or module import paths.
// Returns a PackageCollection containing all matching packages that don't match any exclude glob.
func SelectPackages(paths, excludeGlobs []string) (*PackageCollection, error) {
	if err := validateLocalPaths(paths); err != nil {
		return nil, err
	}

	pkgs, err := PackageInfo(paths...)
	if err != nil {
		return nil, err
	}

	collection := NewPackageCollection(pkgs...)

pkgs:
	for _, p := range pkgs {
		for _, e := range excludeGlobs {
			match, err := doublestar.Match(e, p.ImportPath)
			if err != nil {
				return nil, fmt.Errorf("exclude glob could not be used %q: %w", e, err)
			}
			if match {
				log.WithFields("pkg", p.ImportPath).Trace("excluding package")
				collection.Remove(p)
				continue pkgs
			}
		}
	}

	return collection, err
}

// validateLocalPaths checks that all local paths exist on disk and are directories.
// Module paths (e.g., "github.com/foo/bar") are skipped and left for `go list` to handle.
func validateLocalPaths(paths []string) error {
	for _, p := range paths {
		if !isLocalPath(p) {
			continue
		}

		// strip "..." suffix for recursive patterns (e.g., "./pkg/..." -> "./pkg")
		dirPath := strings.TrimSuffix(p, "...")
		dirPath = strings.TrimSuffix(dirPath, "/")
		if dirPath == "" {
			dirPath = "."
		}

		info, err := os.Stat(dirPath)
		if os.IsNotExist(err) {
			return fmt.Errorf("no such directory: %s", dirPath)
		}
		if err != nil {
			return fmt.Errorf("unable to access path %s: %w", dirPath, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", dirPath)
		}
	}
	return nil
}

// isLocalPath returns true if the path refers to a local filesystem path
// (as opposed to a module path like "github.com/foo/bar").
func isLocalPath(p string) bool {
	return strings.HasPrefix(p, "./") ||
		strings.HasPrefix(p, "../") ||
		strings.HasPrefix(p, "/") ||
		p == "." ||
		p == ".."
}
