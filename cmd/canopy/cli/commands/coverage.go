package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"

	"github.com/anchore/clio"
	"github.com/anchore/fangs"
)

const sortByPercent = "percent"

var _ fangs.FlagAdder = (*coverageConfig)(nil)

// coverageConfig holds all configuration options for the coverage command.
type coverageConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`
	options.Store  `yaml:"store" json:"store" mapstructure:"store"`

	// RunID is the optional run ID from command argument.
	RunID string `yaml:"-" json:"-" mapstructure:"-"`
	// Unit controls the detail unit for table output: total, package, function.
	Unit string `yaml:"unit" json:"unit" mapstructure:"unit"`
	// Output controls the output format: text or json.
	Output string `yaml:"output" json:"output" mapstructure:"output"`
	// Sort controls sort order: name or percent.
	Sort string `yaml:"sort" json:"sort" mapstructure:"sort"`
	// Desc sorts descending.
	Desc bool `yaml:"desc" json:"desc" mapstructure:"desc"`
	// Min filters to items with coverage >= threshold. -1 means not set.
	Min float64 `yaml:"min" json:"min" mapstructure:"min"`
	// Max filters to items with coverage <= threshold. -1 means not set.
	Max float64 `yaml:"max" json:"max" mapstructure:"max"`
	// Package filters to packages matching glob pattern.
	Package string `yaml:"package" json:"package" mapstructure:"package"`
}

func (o *coverageConfig) AddFlags(flags fangs.FlagSet) {
	flags.StringVarP(&o.Unit, "unit", "u", "detail unit: total, package, function")
	flags.StringVarP(&o.Output, "output", "o", "output format: text, json")
	flags.StringVarP(&o.Sort, "sort", "s", "sort by: name, percent")
	flags.BoolVarP(&o.Desc, "desc", "", "sort descending")
	flags.Float64VarP(&o.Min, "min", "", "only show items with coverage >= threshold")
	flags.Float64VarP(&o.Max, "max", "", "only show items with coverage <= threshold")
	flags.StringVarP(&o.Package, "package", "p", "filter to packages matching glob pattern")
}

func defaultCoverageOptions() *coverageConfig {
	store := options.DefaultStore()
	store.Enabled = true
	store.HideEnabledFlag = true

	return &coverageConfig{
		Store:  store,
		Unit:   "package",
		Output: "text",
		Sort:   "name",
		Min:    -1, // sentinel for "not set"
		Max:    -1, // sentinel for "not set"
	}
}

// Coverage creates a command to display coverage data for a test run.
func Coverage(app clio.Application) *cobra.Command {
	opts := defaultCoverageOptions()

	cmd := &cobra.Command{
		Use:   "coverage [RUN-ID]",
		Short: "Show coverage data for the last run (or a specific run)",
		Long: `Display coverage data for a test run at various levels of detail.

By default, shows coverage from the last test run. Optionally specify a run ID
to view coverage for a specific run.

Examples:
  canopy coverage                    # show package-level coverage for last run
  canopy coverage --unit total       # show total coverage only
  canopy coverage --unit function    # show function-level coverage
  canopy coverage abc123             # show coverage for a specific run
  canopy coverage -o json            # JSON output for scripting
  canopy coverage --max 50           # find poorly covered packages (< 50%)
  canopy coverage --min 80           # find well covered packages (>= 80%)
  canopy coverage --sort percent     # sort by coverage percentage
  canopy coverage -p './internal/...'# filter to specific packages`,
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.RunID = args[0]
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runCoverage(*opts)
		},
	}

	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupCommand(cmd, opts)
}

// coverageOutput represents the full JSON output structure.
type coverageOutput struct {
	RunID       string          `json:"run_id"`
	Timestamp   time.Time       `json:"timestamp"`
	CovdataPath string          `json:"covdata_path"`
	Total       coverageTotal   `json:"total"`
	Packages    []packageOutput `json:"packages"`
}

type coverageTotal struct {
	Percent float64 `json:"percent"`
}

type packageOutput struct {
	Path      string           `json:"path"`
	Percent   float64          `json:"percent"`
	Functions []functionOutput `json:"functions"`
}

type functionOutput struct {
	Name    string  `json:"name"`
	File    string  `json:"file"`
	Line    int     `json:"line"`
	Percent float64 `json:"percent"`
}

// coverageData holds all fetched coverage information for a single run.
type coverageData struct {
	runInfo      test.RunInfo
	testRun      db.TestRun
	pkgCoverage  []db.PackageCoverage
	funcCoverage []db.FunctionCoverage
}

func loadCoverageData(cfg coverageConfig) (*coverageData, error) {
	m, err := test.NewManager(
		test.Config{
			DBRoot:    cfg.Root,
			Ephemeral: cfg.Ephemeral,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create test manager: %w", err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			log.WithFields("error", err).Error("unable to close test manager")
		}
	}()

	runID, err := resolveRunID(m, cfg.RunID)
	if err != nil {
		return nil, err
	}

	log.WithFields("run-id", runID.String()).Debug("showing coverage for test run")

	runInfo, err := m.GetRunInfo(runID)
	if err != nil {
		return nil, fmt.Errorf("unable to get run info: %w", err)
	}

	store := m.DBStore()
	if store == nil {
		return nil, fmt.Errorf("database not available")
	}

	testRun, err := store.GetTestRun(runID)
	if err != nil {
		return nil, fmt.Errorf("unable to get test run: %w", err)
	}

	if runInfo.Coverage == nil {
		return nil, fmt.Errorf("no coverage data available for this run (was coverage enabled?)")
	}

	pkgCoverage, err := store.GetPackageCoverage(runID)
	if err != nil {
		return nil, fmt.Errorf("unable to get package coverage: %w", err)
	}

	funcCoverage, err := store.GetFunctionCoverage(runID)
	if err != nil {
		return nil, fmt.Errorf("unable to get function coverage: %w", err)
	}

	return &coverageData{
		runInfo:      runInfo,
		testRun:      testRun,
		pkgCoverage:  pkgCoverage,
		funcCoverage: funcCoverage,
	}, nil
}

func runCoverage(cfg coverageConfig) error {
	data, err := loadCoverageData(cfg)
	if err != nil {
		return err
	}

	// convert sentinel to nil for optional min/max
	var minPct, maxPct *float64
	if cfg.Min >= 0 {
		minPct = &cfg.Min
	}
	if cfg.Max >= 0 {
		maxPct = &cfg.Max
	}

	// filter and sort
	pkgs := filterPackages(data.pkgCoverage, cfg.Package, minPct, maxPct)
	funcs := filterFunctions(data.funcCoverage, cfg.Package, minPct, maxPct)
	sortPackages(pkgs, cfg.Sort, cfg.Desc)
	sortFunctions(funcs, cfg.Sort, cfg.Desc)

	switch strings.ToLower(cfg.Output) {
	case "json":
		return writeCoverageJSON(os.Stdout, data.runInfo, data.testRun, pkgs, funcs)
	case "text", "":
		return writeCoverageText(os.Stdout, cfg.Unit, data.runInfo, pkgs, funcs, minPct, maxPct)
	default:
		return fmt.Errorf("unknown output format: %s", cfg.Output)
	}
}

func filterPackages(pkgs []db.PackageCoverage, pattern string, minPct, maxPct *float64) []db.PackageCoverage {
	var result []db.PackageCoverage
	for _, pkg := range pkgs {
		if pattern != "" && !matchPackagePattern(pattern, pkg.PackagePath) {
			continue
		}
		if minPct != nil && pkg.Percent < *minPct {
			continue
		}
		if maxPct != nil && pkg.Percent > *maxPct {
			continue
		}
		result = append(result, pkg)
	}
	return result
}

func filterFunctions(funcs []db.FunctionCoverage, pattern string, minPct, maxPct *float64) []db.FunctionCoverage {
	var result []db.FunctionCoverage
	for _, fn := range funcs {
		pkgPath := extractPackagePath(fn.FilePath)

		if pattern != "" && !matchPackagePattern(pattern, pkgPath) {
			continue
		}
		if minPct != nil && fn.Percent < *minPct {
			continue
		}
		if maxPct != nil && fn.Percent > *maxPct {
			continue
		}
		result = append(result, fn)
	}
	return result
}

// matchPackagePattern checks if the value matches the pattern using glob-style matching.
// supports "..." suffix for recursive matching (e.g., "./internal/..." matches all internal packages).
func matchPackagePattern(pattern, value string) bool {
	// handle ... suffix (like ./cmd/... or github.com/org/repo/internal/...)
	if strings.HasSuffix(pattern, "...") {
		prefix := strings.TrimSuffix(pattern, "...")
		prefix = strings.TrimPrefix(prefix, "./")
		return strings.HasPrefix(value, prefix) || strings.Contains(value, "/"+prefix)
	}

	// try filepath.Match for glob patterns
	matched, err := filepath.Match(pattern, value)
	if err != nil {
		// if pattern is invalid, try substring match
		return strings.Contains(value, pattern)
	}
	return matched
}

// extractPackagePath extracts the package path from a file path.
// e.g., "github.com/org/repo/pkg/file.go" -> "github.com/org/repo/pkg"
func extractPackagePath(filePath string) string {
	dir := filepath.Dir(filePath)
	return dir
}

func sortPackages(pkgs []db.PackageCoverage, sortBy string, desc bool) {
	sort.Slice(pkgs, func(i, j int) bool {
		var less bool
		switch sortBy {
		case sortByPercent:
			less = pkgs[i].Percent < pkgs[j].Percent
		default: // name
			less = pkgs[i].PackagePath < pkgs[j].PackagePath
		}
		if desc {
			return !less
		}
		return less
	})
}

func sortFunctions(funcs []db.FunctionCoverage, sortBy string, desc bool) {
	sort.Slice(funcs, func(i, j int) bool {
		var less bool
		switch sortBy {
		case sortByPercent:
			less = funcs[i].Percent < funcs[j].Percent
		default: // name
			// sort by package path then function name
			if funcs[i].FilePath != funcs[j].FilePath {
				less = funcs[i].FilePath < funcs[j].FilePath
			} else {
				less = funcs[i].FuncName < funcs[j].FuncName
			}
		}
		if desc {
			return !less
		}
		return less
	})
}

func writeCoverageJSON(w io.Writer, runInfo test.RunInfo, testRun db.TestRun, pkgs []db.PackageCoverage, funcs []db.FunctionCoverage) error {
	// group functions by package
	funcsByPkg := make(map[string][]db.FunctionCoverage)
	for _, fn := range funcs {
		pkgPath := extractPackagePath(fn.FilePath)
		funcsByPkg[pkgPath] = append(funcsByPkg[pkgPath], fn)
	}

	// build output structure
	output := coverageOutput{
		RunID:       runInfo.UUID.String(),
		Timestamp:   runInfo.Started,
		CovdataPath: testRun.CoverageDir,
		Total: coverageTotal{
			Percent: safePercent(runInfo.Coverage),
		},
		Packages: make([]packageOutput, 0, len(pkgs)),
	}

	for _, pkg := range pkgs {
		pkgOut := packageOutput{
			Path:      pkg.PackagePath,
			Percent:   pkg.Percent,
			Functions: make([]functionOutput, 0),
		}

		if fns, ok := funcsByPkg[pkg.PackagePath]; ok {
			for _, fn := range fns {
				pkgOut.Functions = append(pkgOut.Functions, functionOutput{
					Name:    fn.FuncName,
					File:    filepath.Base(fn.FilePath),
					Line:    fn.Line,
					Percent: fn.Percent,
				})
			}
		}

		output.Packages = append(output.Packages, pkgOut)
	}

	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)

	if err := enc.Encode(output); err != nil {
		return fmt.Errorf("unable to encode coverage as JSON: %w", err)
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func writeCoverageText(w io.Writer, unit string, runInfo test.RunInfo, pkgs []db.PackageCoverage, funcs []db.FunctionCoverage, minPct, maxPct *float64) error {
	switch strings.ToLower(unit) {
	case "total":
		writeTotalCoverage(w, runInfo)
	case "function":
		writeFunctionCoverage(w, runInfo, funcs, minPct, maxPct)
	case "package", "":
		writePackageCoverage(w, runInfo, pkgs)
	default:
		return fmt.Errorf("unknown unit: %s (expected: total, package, function)", unit)
	}
	return nil
}

func writeTotalCoverage(w io.Writer, runInfo test.RunInfo) {
	pct := safePercent(runInfo.Coverage)
	fmt.Fprintf(w, "Coverage: %.1f%%\n", pct)
}

func writePackageCoverage(w io.Writer, runInfo test.RunInfo, pkgs []db.PackageCoverage) {
	if len(pkgs) == 0 {
		fmt.Fprintln(w, "No package coverage data available")
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = true

	t.AppendHeader(table.Row{"PACKAGE", "COVERAGE"})

	for _, pkg := range pkgs {
		t.AppendRow(table.Row{
			pkg.PackagePath,
			fmt.Sprintf("%.1f%%", pkg.Percent),
		})
	}

	// add separator and total
	t.AppendSeparator()
	t.AppendRow(table.Row{
		"TOTAL",
		fmt.Sprintf("%.1f%%", safePercent(runInfo.Coverage)),
	})

	t.Render()
}

func writeFunctionCoverage(w io.Writer, runInfo test.RunInfo, funcs []db.FunctionCoverage, minPct, maxPct *float64) {
	if len(funcs) == 0 {
		fmt.Fprintln(w, "No function coverage data available")
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = true

	t.AppendHeader(table.Row{"FUNCTION", "COVERAGE", "FILE:LINE"})

	for _, fn := range funcs {
		pkgPath := extractPackagePath(fn.FilePath)
		fullName := fmt.Sprintf("%s.%s", pkgPath, fn.FuncName)

		t.AppendRow(table.Row{
			fullName,
			fmt.Sprintf("%.1f%%", fn.Percent),
			fmt.Sprintf("%s:%d", filepath.Base(fn.FilePath), fn.Line),
		})
	}

	t.AppendSeparator()

	// show appropriate footer based on filters
	if minPct != nil || maxPct != nil {
		var footer string
		switch {
		case maxPct != nil && minPct == nil:
			footer = fmt.Sprintf("%d functions at or below %.0f%% coverage", len(funcs), *maxPct)
		case minPct != nil && maxPct == nil:
			footer = fmt.Sprintf("%d functions at or above %.0f%% coverage", len(funcs), *minPct)
		default:
			footer = fmt.Sprintf("%d functions between %.0f%% and %.0f%% coverage", len(funcs), *minPct, *maxPct)
		}
		t.AppendRow(table.Row{footer, "", ""})
	} else {
		t.AppendRow(table.Row{
			"TOTAL",
			fmt.Sprintf("%.1f%%", safePercent(runInfo.Coverage)),
			"",
		})
	}

	t.Render()
}

func safePercent(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
