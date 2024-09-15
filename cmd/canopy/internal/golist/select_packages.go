package golist

import (
	"fmt"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
)

func SelectPackages(paths, excludeGlobs []string) (*PackageCollection, error) {
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
