package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/scylladb/go-set/strset"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/source"

	"github.com/anchore/clio"
	"github.com/anchore/fangs"
)

var _ fangs.FlagAdder = (*affectedConfig)(nil)

// affectedConfig holds configuration for the affected command.
type affectedConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`

	// Specifiers are the Go package patterns to scope analysis to (positional args).
	Specifiers []string `yaml:"packages" json:"packages" mapstructure:"packages"`
	// ExcludePatterns are glob patterns of package paths to ignore.
	ExcludePatterns []string `yaml:"exclude" json:"exclude" mapstructure:"exclude"`
	// Since is a git ref; when set, changed files are computed against it.
	Since string `yaml:"since" json:"since" mapstructure:"since"`
	// Files is an explicit CSV list of changed .go files (overrides git detection).
	Files []string `yaml:"files" json:"files" mapstructure:"files"`
	// Output controls the output format: table or json.
	Output string `yaml:"output" json:"output" mapstructure:"output"`
	// ShowPackages groups the affected tests under their package in table output.
	ShowPackages bool `yaml:"show-packages" json:"show-packages" mapstructure:"show-packages"`
}

func (o *affectedConfig) AddFlags(flags fangs.FlagSet) {
	flags.StringVarP(&o.Since, "since", "", "git ref to compute changed files against (e.g. HEAD~3, main)")
	flags.StringArrayVarP(&o.Files, "files", "", "explicit changed .go files (comma-separated), overrides git detection")
	flags.StringVarP(&o.Output, "output", "o", "output format: table, json")
	flags.StringArrayVarP(&o.ExcludePatterns, "exclude", "e", "glob patterns of package paths to ignore")
	flags.BoolVarP(&o.ShowPackages, "show-packages", "", "group affected tests under their package (changed packages highlighted)")
}

// Affected creates a command that reports the minimal sound set of packages and tests to run
// to verify a change, using the static Go import graph.
func Affected(app clio.Application) *cobra.Command {
	opts := &affectedConfig{
		Specifiers: []string{options.DefaultPackageSpecifier},
		Output:     formatTable,
	}

	cmd := &cobra.Command{
		Use:   "affected [GO-PKG-SPECIFIER...]",
		Short: "report tests affected by a change using the static import graph",
		Long: `Determine the minimal sound set of tests to run to verify a change did not break
something, using only the static Go import graph (no coverage).

The analysis is a deliberate over-approximation: it will never drop a possibly-affected
test, but it may include tests that a change cannot actually break. A package is affected if
it changed, if it (transitively) imports a changed package, or if its tests import one.

Changed files are detected from (in priority order): --files, --since, or the dirty working
tree (staged, unstaged, and untracked).

Examples:
  # tests affected by uncommitted changes
  canopy affected

  # tests affected since a git ref
  canopy affected --since HEAD~3

  # tests affected by specific files
  canopy affected --files handler.go,session.go

  # scope analysis to a subtree
  canopy affected ./internal/...

  # group the affected tests under their package
  canopy affected --show-packages

  # JSON output for agent/CI consumption (includes changed files and packages)
  canopy affected -o json`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Specifiers = args
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runAffected(*opts)
		},
	}

	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupCommand(cmd, opts)
}

// affectedResult is the computed impact analysis, shared by table and JSON rendering.
type affectedResult struct {
	Reason           string   `json:"reason"`
	ChangedFiles     []string `json:"changed_files"`
	ChangedPackages  []string `json:"changed_packages"`
	AffectedPackages []string `json:"affected_packages"`
	Tests            []string `json:"tests"`
}

func runAffected(cfg affectedConfig) error {
	changedFiles, err := resolveChangedFiles(cfg)
	if err != nil {
		return err
	}

	// load the import graph for the scoped packages
	pkgs, err := golist.PackageGraph(cfg.Specifiers...)
	if err != nil {
		return fmt.Errorf("unable to list packages: %w", err)
	}

	// map changed files (by directory) to their package import paths
	changed := changedImportPaths(changedFiles, pkgs)

	affected := affectedPackages(pkgs, changed)

	// discover test functions in affected packages (function-level references)
	refs, err := affectedTests(pkgs, affected)
	if err != nil {
		return fmt.Errorf("unable to discover affected tests: %w", err)
	}

	tests := make([]string, len(refs))
	for i, r := range refs {
		tests[i] = r.String(false)
	}

	result := affectedResult{
		Reason:           "import-graph",
		ChangedFiles:     relativizeAll(changedFiles),
		ChangedPackages:  sortedList(changed),
		AffectedPackages: sortedList(affected),
		Tests:            tests,
	}

	if strings.ToLower(cfg.Output) == formatJSON {
		return writeAffectedJSON(result)
	}
	writeAffectedTable(result, refs, changed, cfg.ShowPackages)
	return nil
}

// computeAffectedImportPaths runs the import-graph impact analysis and returns the affected
// package import paths, scoped to the given specifiers. Changed files come from the git ref
// when `since` is set, otherwise the dirty working tree. Shared by `canopy affected` and the
// `--affected` flag on `test`/root, which need only the package set (not the full report).
// Exclude patterns are intentionally not applied here: callers feed this set back through
// golist.SelectPackages, which applies excludes to the final run set.
func computeAffectedImportPaths(specifiers []string, since string) (*strset.Set, error) {
	var (
		changedFiles []string
		err          error
	)
	if since != "" {
		changedFiles, err = source.ChangedGoFilesSince(".", since)
	} else {
		changedFiles, err = source.ChangedGoFiles(".")
	}
	if err != nil {
		return nil, err
	}

	pkgs, err := golist.PackageGraph(specifiers...)
	if err != nil {
		return nil, fmt.Errorf("unable to list packages: %w", err)
	}

	changed := changedImportPaths(changedFiles, pkgs)
	return affectedPackages(pkgs, changed), nil
}

// resolveChangedFiles determines the set of changed .go files (absolute paths) from the
// configured source, in priority order: explicit files, git ref, then dirty working tree.
func resolveChangedFiles(cfg affectedConfig) ([]string, error) {
	switch {
	case len(cfg.Files) > 0:
		return explicitFiles(cfg.Files), nil
	case cfg.Since != "":
		return source.ChangedGoFilesSince(".", cfg.Since)
	default:
		return source.ChangedGoFiles(".")
	}
}

// explicitFiles splits comma-separated file arguments, keeps only .go files, and makes
// each path absolute so it can be matched against package directories.
func explicitFiles(args []string) []string {
	var out []string
	for _, arg := range args {
		for f := range strings.SplitSeq(arg, ",") {
			f = strings.TrimSpace(f)
			if f == "" || !strings.HasSuffix(f, ".go") {
				continue
			}
			if abs, err := filepath.Abs(f); err == nil {
				out = append(out, abs)
			} else {
				out = append(out, f)
			}
		}
	}
	return out
}

// changedImportPaths maps changed .go files to the import paths of the packages that own them,
// by matching each file's directory against the packages' directories.
func changedImportPaths(files []string, pkgs []golist.Package) *strset.Set {
	dirToPkg := make(map[string]string, len(pkgs))
	for _, p := range pkgs {
		dirToPkg[p.Dir] = p.ImportPath
	}

	changed := strset.New()
	for _, f := range files {
		if importPath, ok := dirToPkg[filepath.Dir(f)]; ok {
			changed.Add(importPath)
		} else {
			log.WithFields("file", f).Trace("changed file does not map to a package in scope")
		}
	}
	return changed
}

// affectedPackages computes the transitive set of packages whose build or tests could be
// impacted by a change to any package in `changed`, using the static import graph. A package
// p is affected if it is itself changed, if a changed package appears in its transitive Deps,
// or if any of its test imports (directly, or transitively via that import's Deps) is changed.
// Deps from `go list` is already transitive, so this is a single pass.
//
// ponytail: import-graph only. Edges introduced by reflection, code generation, build tags
// not in the active config, or //go:embed are invisible here; the analysis intentionally
// over-approximates (never drops a possibly-affected test) rather than guessing. Upgrade path
// is coverage-based line attribution (see specs/vision/test-impact-analysis) if precision is
// ever needed over soundness.
func affectedPackages(pkgs []golist.Package, changed *strset.Set) *strset.Set {
	// index each package's transitive build deps for expanding direct test imports one hop
	depsByPkg := make(map[string][]string, len(pkgs))
	for _, p := range pkgs {
		depsByPkg[p.ImportPath] = p.Deps
	}

	affected := strset.New()
	for _, p := range pkgs {
		switch {
		case changed.Has(p.ImportPath):
			affected.Add(p.ImportPath)
		case changed.HasAny(p.Deps...):
			affected.Add(p.ImportPath)
		case testImportsHitChanged(p, depsByPkg, changed):
			affected.Add(p.ImportPath)
		}
	}
	return affected
}

// testImportsHitChanged reports whether any of p's test imports is a changed package, or
// (transitively) depends on one. Test imports are direct-only in `go list`, so each is expanded
// one hop through its own transitive Deps.
func testImportsHitChanged(p golist.Package, depsByPkg map[string][]string, changed *strset.Set) bool {
	for _, ti := range append(append([]string{}, p.TestImports...), p.XTestImports...) {
		if changed.Has(ti) || changed.HasAny(depsByPkg[ti]...) {
			return true
		}
	}
	return false
}

// affectedTests discovers the function-level test references in the affected packages,
// deduped and sorted by their fully-qualified reference.
func affectedTests(pkgs []golist.Package, affected *strset.Set) ([]gotest.Reference, error) {
	var affectedPkgs []golist.Package
	for _, p := range pkgs {
		if affected.Has(p.ImportPath) {
			affectedPkgs = append(affectedPkgs, p)
		}
	}
	if len(affectedPkgs) == 0 {
		return nil, nil
	}

	collection := golist.NewPackageCollection(affectedPkgs...)
	defs, err := gotest.FindDefinitions(collection)
	if err != nil {
		return nil, err
	}

	// dedupe to function-level references (skip subtests to keep the list lean)
	seen := strset.New()
	var refs []gotest.Reference
	for _, d := range defs {
		ref := gotest.Reference{Package: d.ImportPath, FuncName: d.FnName}
		key := ref.String(false)
		if seen.Has(key) {
			continue
		}
		seen.Add(key)
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].String(false) < refs[j].String(false)
	})
	return refs, nil
}

func sortedList(s *strset.Set) []string {
	out := s.List()
	sort.Strings(out)
	return out
}

// relativizeAll converts absolute file paths to paths relative to the working directory for
// display, falling back to the absolute path when relativization fails.
func relativizeAll(files []string) []string {
	cwd, err := os.Getwd()
	if err != nil {
		return files
	}
	out := make([]string, len(files))
	for i, f := range files {
		if rel, err := filepath.Rel(cwd, f); err == nil {
			out[i] = rel
		} else {
			out[i] = f
		}
	}
	return out
}

func writeAffectedJSON(result affectedResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(result)
}

// output styles shared by triage and affected so the two commands speak one visual language:
// the same color always means the same thing. styleChange = new/attributable to the current
// change (a new-regression failure; a modified package); styleCaution = pre-existing; styleFlaky
// = intermittent anomaly; stylePackage = a package/grouping label; styleAux = de-emphasized
// detail. lipgloss strips ANSI off-terminal, so piped output stays clean.
var (
	styleChange  = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)  // red
	styleCaution = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))            // yellow
	styleFlaky   = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true) // magenta
	stylePackage = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))            // teal
	styleAux     = lipgloss.NewStyle().Faint(true)
)

func writeAffectedTable(result affectedResult, refs []gotest.Reference, changed *strset.Set, showPackages bool) {
	if len(result.ChangedFiles) == 0 {
		fmt.Println("no changed .go files detected")
		return
	}
	if len(refs) == 0 {
		fmt.Println("no affected tests")
		return
	}

	pkgs := strset.New()
	for _, r := range refs {
		pkgs.Add(r.Package)
	}

	if showPackages {
		writeTestsByPackage(refs, changed)
	} else {
		for _, r := range refs {
			fmt.Println(r.String(false))
		}
	}

	// rollup footer: how many changed packages actually carry affected tests
	changedWithTests := 0
	for _, p := range pkgs.List() {
		if changed.Has(p) {
			changedWithTests++
		}
	}
	summary := fmt.Sprintf("%d affected tests across %d packages (%d changed)", len(refs), pkgs.Size(), changedWithTests)
	fmt.Fprintf(os.Stderr, "\n%s\n", styleAux.Render(summary))
}

// writeTestsByPackage prints each affected test grouped under its package header. Package
// headers are styled to distinguish changed from transitively-affected; a literal "(changed)"
// tag keeps the distinction legible when color is stripped.
func writeTestsByPackage(refs []gotest.Reference, changed *strset.Set) {
	byPkg := map[string][]gotest.Reference{}
	var order []string
	for _, r := range refs {
		if _, ok := byPkg[r.Package]; !ok {
			order = append(order, r.Package)
		}
		byPkg[r.Package] = append(byPkg[r.Package], r)
	}
	sort.Strings(order)

	for _, pkg := range order {
		if changed.Has(pkg) {
			// delta glyph aligns the package name with the leading pad of affected rows
			fmt.Printf("%s %s %s\n", styleChange.Render("Δ"), stylePackage.Render(pkg), styleAux.Render("(modified)"))
		} else {
			fmt.Printf("  %s\n", stylePackage.Render(pkg))
		}
		for _, r := range byPkg[pkg] {
			fmt.Printf("    %s\n", r.TestName(false))
		}
	}
}
