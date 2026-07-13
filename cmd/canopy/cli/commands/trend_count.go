package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/trend"

	"github.com/anchore/clio"
)

type countConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`
	options.Store  `yaml:"store" json:"store" mapstructure:"store"`
	options.Trend  `yaml:"trend" json:"trend" mapstructure:"trend"`
}

// TrendCount creates a command to report how the size of the test suite has changed
// over historical runs.
func TrendCount(app clio.Application) *cobra.Command {
	store := options.DefaultStore()
	store.Enabled = true
	opts := &countConfig{
		Store: store,
		Trend: options.DefaultTrend(),
	}

	cmd := &cobra.Command{
		Use:   "count [GO-PKG-SPECIFIER...]",
		Short: "report test suite size over time",
		Long: `Track how many distinct tests ran across historical sessions.

Each run contributes one point: the number of distinct per-test references observed
and how many packages they span. A summary line reports the overall change and the
package breadth across the window.

Counts are only comparable run-to-run when the scope is stable, so the counts reflect
whatever each run happened to select and execute, not the whole suite. Use a package
filter (as with 'trend duration') to pin the scope, then the point column tells you how
many packages you're actually looking across.

Caching caveat: a cached package emits a single "(cached)" event and no per-test
events, so a cached run reports zero tests for that package and undercounts the total.
Counts are only trustworthy on runs executed with caching disabled (go test -count=1);
mixed-cache history will show artificial dips.

Examples:
  # Suite size across the most recent runs
  canopy trend count

  # Only specific packages
  canopy trend count ./cmd/canopy/...

  # Exclude packages
  canopy trend count ./... --exclude '**/vendor/*'

  # Only tests matching a pattern
  canopy trend count --test 'TestUser.*'

  # Limit to a time window
  canopy trend count --window 168h

  # Analyze specific sessions
  canopy trend count --session last`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Specifiers = args
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runCount(*opts)
		},
	}

	// facilitates grouping of flags into sections in help text
	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupCommand(cmd, opts)
}

func runCount(cfg countConfig) error {
	log.Info("analyzing test suite size from historical sessions")

	records, err := collectTrendRecords(cfg.Store, cfg.Trend)
	if err != nil {
		return err
	}

	points := trend.Counts(records)

	if len(points) == 0 {
		if options.TrendOutputFormat(cfg.Output) == options.TrendOutputJSON {
			fmt.Println("[]")
		} else {
			fmt.Println("No runs found for the given scope.")
		}
		return nil
	}

	switch options.TrendOutputFormat(cfg.Output) {
	case options.TrendOutputJSON:
		return displayCountResultsJSON(points)
	default:
		displayCountResults(points, trend.DistinctPackages(records))
	}
	return nil
}

// displayCountResults renders the suite-size series and a delta summary to stdout.
// totalPackages is the distinct package breadth across the whole window: counts are
// only comparable run-to-run when that breadth (and caching) holds steady, so scoping
// with a package filter is the intended way to make this meaningful.
func displayCountResults(points []trend.CountPoint, totalPackages int) {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = false

	t.AppendHeader(table.Row{"Run", "Tests", "Packages"})

	for _, p := range points {
		t.AppendRow(table.Row{
			formatRunLabel(p),
			p.TestCount,
			p.PackageCount,
		})
	}

	fmt.Println(t.Render())

	d := trend.Delta(points)
	printRollupf("%d → %d tests (%+d) across %d package(s) over %d run(s)",
		d.First, d.Last, d.Change(), totalPackages, d.Runs)
}

// countPointJSON represents one point in the suite-size series in JSON format.
type countPointJSON struct {
	RunID        string    `json:"run_id"`
	Time         time.Time `json:"time"`
	TestCount    int       `json:"test_count"`
	PackageCount int       `json:"package_count"`
}

// displayCountResultsJSON renders the suite-size series as JSON to stdout.
func displayCountResultsJSON(points []trend.CountPoint) error {
	output := make([]countPointJSON, 0, len(points))
	for _, p := range points {
		output = append(output, countPointJSON{
			RunID:        p.RunID.String(),
			Time:         p.Time,
			TestCount:    p.TestCount,
			PackageCount: p.PackageCount,
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// formatRunLabel labels a run by its relative time, falling back to a short id.
func formatRunLabel(p trend.CountPoint) string {
	if !p.Time.IsZero() {
		return formatRelativeTime(p.Time)
	}
	return p.RunID.String()[:8]
}
