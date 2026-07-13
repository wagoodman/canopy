package golist

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
)

// packageInfoTemplate is the `go list -f` template that emits one JSON object per package.
// Fields are rendered with %q so paths containing backslashes (all Windows paths) or quotes
// stay valid JSON, and Module is guarded since it's nil for stdlib/GOPATH packages.
const packageInfoTemplate = `{ "Dir": {{printf "%q" .Dir}}, "ImportPath": {{printf "%q" .ImportPath}}, "ModulePath": {{if .Module}}{{printf "%q" .Module.Path}}{{else}}""{{end}} }`

// derived from "go help list" output

// PackageCollection maintains an ordered collection of packages with efficient lookup by import path and directory.
// It preserves insertion order and provides bidirectional mapping between package paths and filesystem directories.
type PackageCollection struct {
	// order tracks the insertion order of package import paths.
	order []string
	// set maps import paths to their Package information.
	set map[string]Package
	// indexByDir maps filesystem directories to their import paths for reverse lookup.
	indexByDir map[string]string
	// module is the Go module path extracted from the first package added to the collection.
	module string
}

// NewPackageCollection creates a new package collection initialized with the given packages.
// Packages are indexed by import path and directory for efficient lookup.
func NewPackageCollection(pkgs ...Package) *PackageCollection {
	c := &PackageCollection{
		set:        make(map[string]Package), // index by package name
		indexByDir: make(map[string]string),
	}

	for _, p := range pkgs {
		c.Add(p)
	}

	return c
}

// Add inserts a package into the collection, maintaining insertion order and updating all indexes.
// The module path is extracted from the first package that provides one.
func (c *PackageCollection) Add(pkg Package) {
	c.order = append(c.order, pkg.ImportPath)
	c.set[pkg.ImportPath] = pkg
	c.indexByDir[pkg.Dir] = pkg.ImportPath
	if c.module == "" && pkg.ModulePath != "" {
		c.module = pkg.ModulePath
	}
}

// Module returns the Go module path for the packages in this collection.
// Returns empty string if no module path has been set.
func (c *PackageCollection) Module() string {
	return c.module
}

// Size returns the number of packages in the collection.
func (c *PackageCollection) Size() int {
	return len(c.set)
}

// Remove deletes a package from the collection, removing it from all indexes and the order list.
func (c *PackageCollection) Remove(pkg Package) {
	delete(c.set, pkg.ImportPath)
	delete(c.indexByDir, pkg.Dir)
	for i, p := range c.order {
		if p == pkg.ImportPath {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// ImportPaths returns a copy of all import paths in insertion order.
func (c *PackageCollection) ImportPaths() []string {
	pkgs := make([]string, len(c.order))
	copy(pkgs, c.order)
	return pkgs
}

// Packages returns all packages in insertion order.
func (c *PackageCollection) Packages() []Package {
	var pkgs []Package
	for _, importPath := range c.order {
		pkgs = append(pkgs, c.set[importPath])
	}
	return pkgs
}

// GetDir returns the filesystem directory for the given import path.
// Returns empty string if the import path is not found.
func (c *PackageCollection) GetDir(importPath string) string {
	pkg, ok := c.set[importPath]
	if !ok {
		return ""
	}
	return pkg.Dir
}

// GetByDir returns the package at the given filesystem directory.
// Returns nil if no package is found at that directory.
func (c *PackageCollection) GetByDir(dir string) *Package {
	importPath, ok := c.indexByDir[dir]
	if !ok {
		return nil
	}

	item := c.set[importPath]
	return &item
}

// Package represents a Go package with its filesystem location and import path.
// This is a minimal subset of the full structure returned by `go list -json`.
// Only the fields needed for test discovery are included.
type Package struct {
	// Dir is the directory containing package sources.
	Dir string
	// ImportPath is the import path of the package.
	ImportPath string
	// ModulePath is the module path containing this package (extracted from Module.Path).
	ModulePath string // module path of package (this is not part of the original datastructure, but instead extracted from Module.Path)
	// Deps lists all (recursively) imported dependencies of the package's non-test build.
	Deps []string
	// TestImports lists imports from in-package _test.go files (TestGoFiles).
	TestImports []string
	// XTestImports lists imports from external _test.go files (XTestGoFiles).
	XTestImports []string
	// ImportComment string   // path in import comment on package statement
	// Name          string   // package name
	// Doc           string   // package documentation string
	// Target        string   // install path
	// Shlib         string   // the shared library that contains this package (only set when -linkshared)
	// Goroot        bool     // is this package in the Go root?
	// Standard      bool     // is this package part of the standard Go library?
	// Stale         bool     // would 'go install' do anything for this package?
	// StaleReason   string   // explanation for Stale==true
	// Root          string   // Go root or Go path dir containing this package
	// ConflictDir   string   // this directory shadows Dir in $GOPATH
	// BinaryOnly    bool     // binary-only package (no longer supported)
	// ForTest       string   // package is only for use in named test
	// Export        string   // file containing export data (when using -export)
	// BuildID       string   // build ID of the compiled package (when using -export)
	// Module        *Module  // info about package's containing module, if any (can be nil)
	// Match         []string // command-line patterns matching this package
	// DepOnly       bool     // package is only a dependency, not explicitly listed
	//
	// // Source files
	// GoFiles           []string // .go source files (excluding CgoFiles, TestGoFiles, XTestGoFiles)
	// CgoFiles          []string // .go source files that import "C"
	// CompiledGoFiles   []string // .go files presented to compiler (when using -compiled)
	// IgnoredGoFiles    []string // .go source files ignored due to build constraints
	// IgnoredOtherFiles []string // non-.go source files ignored due to build constraints
	// CFiles            []string // .c source files
	// CXXFiles          []string // .cc, .cxx and .cpp source files
	// MFiles            []string // .m source files
	// HFiles            []string // .h, .hh, .hpp and .hxx source files
	// FFiles            []string // .f, .F, .for and .f90 Fortran source files
	// SFiles            []string // .s source files
	// SwigFiles         []string // .swig files
	// SwigCXXFiles      []string // .swigcxx files
	// SysoFiles         []string // .syso object files to add to archive
	// TestGoFiles       []string // _test.go files in package
	// XTestGoFiles      []string // _test.go files outside package
	//
	// // Embedded files
	// EmbedPatterns      []string // //go:embed patterns
	// EmbedFiles         []string // files matched by EmbedPatterns
	// TestEmbedPatterns  []string // //go:embed patterns in TestGoFiles
	// TestEmbedFiles     []string // files matched by TestEmbedPatterns
	// XTestEmbedPatterns []string // //go:embed patterns in XTestGoFiles
	// XTestEmbedFiles    []string // files matched by XTestEmbedPatterns
	//
	// // Cgo directives
	// CgoCFLAGS    []string // cgo: flags for C compiler
	// CgoCPPFLAGS  []string // cgo: flags for C preprocessor
	// CgoCXXFLAGS  []string // cgo: flags for C++ compiler
	// CgoFFLAGS    []string // cgo: flags for Fortran compiler
	// CgoLDFLAGS   []string // cgo: flags for linker
	// CgoPkgConfig []string // cgo: pkg-config names
	//
	// // Dependency information
	// Imports      []string          // import paths used by this package
	// ImportMap    map[string]string // map from source import to ImportPath (identity entries omitted)
	// Deps         []string          // all (recursively) imported dependencies
	// TestImports  []string          // imports from TestGoFiles
	// XTestImports []string          // imports from XTestGoFiles
	//
	// // Error information
	// Incomplete bool            // this package or a dependency has an error
	// Error      *PackageError   // error loading package
	// DepsErrors []*PackageError // errors loading dependencies
}

// type PackageError struct {
//	ImportStack []string // shortest path from package named on command line to this one
//	Pos         string   // position of error (if present, file:line:col)
//	Err         string   // the error itself
// }
//
// type Module struct {
//	Path       string       // module path
//	Query      string       // version query corresponding to this version
//	Version    string       // module version
//	Versions   []string     // available module versions
//	Replace    *Module      // replaced by this module
//	Time       *time.Time   // time version was created
//	Update     *Module      // available update (with -u)
//	Main       bool         // is this the main module?
//	Indirect   bool         // module is only indirectly needed by main module
//	Dir        string       // directory holding local copy of files, if any
//	GoMod      string       // path to go.mod file describing module, if any
//	GoVersion  string       // go version used in module
//	Retracted  []string     // retraction information, if any (with -retracted or -u)
//	Deprecated string       // deprecation message, if any (with -u)
//	Error      *ModuleError // error loading module
//	Origin     any          // provenance of module
//	Reuse      bool         // reuse of old module info is safe
// }
//
// type ModuleError struct {
//	Err string // the error itself
// }

// PackageInfo retrieves detailed package information for the given package patterns using `go list`.
// It returns a slice of Package structs containing directory, import path, and module information.
// The implementation uses a minimal JSON template for performance rather than full `-json` output.
func PackageInfo(pkgs ...string) ([]Package, error) {
	var output []Package

	fn := func(stdout io.ReadCloser) error {
		reader := bufio.NewReader(stdout)
		for {
			line, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				log.WithFields("error", err).Warn("error reading from go list stdout")
				return err
			}

			if line == "" {
				break
			}

			// assume JSONL entry...
			var pkg Package
			if err := json.NewDecoder(strings.NewReader(line)).Decode(&pkg); err != nil {
				log.WithFields("error", err).Warn("error decoding go list entry")
				return err
			}
			output = append(output, pkg)
		}
		return nil
	}

	// note: doing -f with minimal fields is much faster than doing -json

	if err := run([]string{"-f", packageInfoTemplate}, fn, pkgs...); err != nil {
		return nil, err
	}

	log.WithFields("count", len(output)).Trace("go list packages")

	return output, nil
}

// goListJSON is the subset of `go list -json` output needed to build the import graph.
// It mirrors the nested Module object that the minimal -f template flattens into ModulePath.
type goListJSON struct {
	Dir          string
	ImportPath   string
	Module       *struct{ Path string }
	Deps         []string
	TestImports  []string
	XTestImports []string
}

// PackageGraph retrieves packages along with their dependency edges (Deps, TestImports,
// XTestImports) using `go list -json`. This is heavier than PackageInfo's minimal template,
// but the extra edges are required for reverse import-graph impact analysis and this is not
// a hot path. Output objects are streamed since `go list -json` emits concatenated (not JSONL)
// JSON documents.
func PackageGraph(pkgs ...string) ([]Package, error) {
	var output []Package

	fn := func(stdout io.ReadCloser) error {
		dec := json.NewDecoder(stdout)
		for {
			var raw goListJSON
			if err := dec.Decode(&raw); err != nil {
				if err == io.EOF {
					break
				}
				log.WithFields("error", err).Warn("error decoding go list -json entry")
				return err
			}
			p := Package{
				Dir:          raw.Dir,
				ImportPath:   raw.ImportPath,
				Deps:         raw.Deps,
				TestImports:  raw.TestImports,
				XTestImports: raw.XTestImports,
			}
			if raw.Module != nil {
				p.ModulePath = raw.Module.Path
			}
			output = append(output, p)
		}
		return nil
	}

	if err := run([]string{"-json"}, fn, pkgs...); err != nil {
		return nil, err
	}

	log.WithFields("count", len(output)).Trace("go list -json packages")

	return output, nil
}
