package golist

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/google/shlex"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
)

// derived from "go help list" output

type PackageCollection struct {
	order      []string
	set        map[string]Package
	indexByDir map[string]string
	module     string
}

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

func (c *PackageCollection) Add(pkg Package) {
	c.order = append(c.order, pkg.ImportPath)
	c.set[pkg.ImportPath] = pkg
	c.indexByDir[pkg.Dir] = pkg.ImportPath
	if c.module == "" && pkg.ModulePath != "" {
		c.module = pkg.ModulePath
	}
}

func (c *PackageCollection) Module() string {
	return c.module
}

func (c *PackageCollection) Size() int {
	return len(c.set)
}

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

func (c *PackageCollection) ImportPaths() []string {
	pkgs := make([]string, len(c.order))
	copy(pkgs, c.order)
	return pkgs
}

func (c *PackageCollection) Packages() []Package {
	var pkgs []Package
	for _, importPath := range c.order {
		pkgs = append(pkgs, c.set[importPath])
	}
	return pkgs
}

func (c *PackageCollection) GetDir(importPath string) string {
	pkg, ok := c.set[importPath]
	if !ok {
		return ""
	}
	return pkg.Dir
}

func (c *PackageCollection) GetByDir(dir string) *Package {
	importPath, ok := c.indexByDir[dir]
	if !ok {
		return nil
	}

	item := c.set[importPath]
	return &item
}

type Package struct {
	Dir        string // directory containing package sources
	ImportPath string // import path of package in dir
	ModulePath string // module path of package (this is not part of the original datastructure, but instead extracted from Module.Path)
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

	args, err := shlex.Split(`-f '{ "Dir": "{{ .Dir }}",  "ImportPath": "{{ .ImportPath }}", "ModulePath": "{{.Module.Path}}" }'`)
	if err != nil {
		return nil, fmt.Errorf("unable to parse go list args: %w", err)
	}

	if err := run(args, fn, pkgs...); err != nil {
		return nil, err
	}

	log.WithFields("count", len(output)).Trace("go list packages")

	return output, nil
}
