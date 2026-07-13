package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/trend"

	"github.com/anchore/clio"
)

type durationConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`
	options.Store  `yaml:"store" json:"store" mapstructure:"store"`
	options.Trend  `yaml:"trend" json:"trend" mapstructure:"trend"`
}

// TrendDuration creates a command to report per-test duration trends from historical sessions.
// It surfaces the slowest tests, their timing history, and how often they were cached.
func TrendDuration(app clio.Application) *cobra.Command {
	store := options.DefaultStore()
	store.Enabled = true
	opts := &durationConfig{
		Store: store,
		Trend: options.DefaultTrend(),
	}

	cmd := &cobra.Command{
		Use:   "duration [GO-PKG-SPECIFIER...]",
		Short: "report test duration trends from historical sessions",
		Long: `Analyze how test durations change over time.

Lists the slowest tests with their latest duration and a history sparkline across
the in-scope runs, plus an overall headline trend for the window.

Only fresh executions carry timing: a cached package emits no per-test events, so a
cached run contributes no duration sample. Where a test's package was cached in some
runs, the history cell notes how many (e.g. "3 cached") so you know how much of the
window actually timed the test. Scope with a package filter to focus the report.

Examples:
  # Duration trends across recent runs
  canopy trend duration

  # Analyze only specific packages
  canopy trend duration ./cmd/canopy/...

  # Exclude specific packages
  canopy trend duration ./... --exclude '**/vendor/*'

  # Only analyze tests matching a pattern
  canopy trend duration --test 'TestUser.*'

  # Only analyze the most recent session
  canopy trend duration --session last

  # Only analyze runs from the last 7 days
  canopy trend duration --window 168h`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Specifiers = args
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDuration(*opts)
		},
	}

	// facilitates grouping of flags into sections in help text
	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupCommand(cmd, opts)
}

func runDuration(cfg durationConfig) error {
	log.Info("analyzing test duration trends from historical sessions")

	records, err := collectTrendRecords(cfg.Store, cfg.Trend)
	if err != nil {
		return err
	}

	results := trend.Durations(records)

	if len(results) == 0 {
		if options.TrendOutputFormat(cfg.Output) == options.TrendOutputJSON {
			fmt.Println("[]")
		} else {
			fmt.Println("No duration data found.")
		}
		return nil
	}

	switch options.TrendOutputFormat(cfg.Output) {
	case options.TrendOutputJSON:
		return displayDurationResultsJSON(results)
	default:
		displayDurationResults(results, trend.CachedRunCount(records))
	}
	return nil
}

// displayDurationResults renders the duration trend results as a table to stdout.
// cachedRuns is the number of in-scope runs that cached at least one package, noted in
// the rollup so a mostly-cached window is not mistaken for missing data.
func displayDurationResults(results []trend.DurationResult, cachedRuns int) {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = false

	t.AppendHeader(table.Row{colTest, "Latest", "History"})

	for _, r := range results {
		hist := sparkline(r.Series)
		if r.CachedRuns > 0 {
			hist += "  " + styleAux.Render(fmt.Sprintf("%d cached", r.CachedRuns))
		}
		t.AppendRow(table.Row{
			r.Reference.String(false),
			formatDuration(r.Latest),
			hist,
		})
	}

	fmt.Println(t.Render())

	overall := trend.OverallDuration(results)
	rollup := fmt.Sprintf("overall %s → %s (%s) across %d test(s)",
		formatDuration(overall.AvgFirst), formatDuration(overall.AvgLast),
		formatTrendPct(overall.TrendPct), overall.Tests)
	if cachedRuns > 0 {
		rollup += fmt.Sprintf("; %d run(s) had cached packages", cachedRuns)
	}
	printRollupf("%s", rollup)
}

// formatTrendPct renders a signed percentage, with an explicit + for regressions.
func formatTrendPct(pct float64) string {
	if pct > 0 {
		return fmt.Sprintf("+%.0f%%", pct)
	}
	return fmt.Sprintf("%.0f%%", pct)
}

// formatDuration renders seconds compactly: milliseconds under a second, else seconds,
// so sub-second tests are legible instead of collapsing to "0.00s".
func formatDuration(sec float64) string {
	switch {
	case sec <= 0:
		return "0ms"
	case sec < 1:
		return fmt.Sprintf("%dms", int(sec*1000+0.5))
	default:
		return fmt.Sprintf("%.2fs", sec)
	}
}

// sparkline renders a series of durations as unicode block bars scaled to the max.
func sparkline(series []float64) string {
	if len(series) == 0 {
		return ""
	}
	bars := []rune("▁▂▃▄▅▆▇█")

	var peak float64
	for _, v := range series {
		if v > peak {
			peak = v
		}
	}
	// all-zero (or single-point) series has no shape; show a flat baseline
	if peak == 0 {
		return strings.Repeat(string(bars[0]), len(series))
	}

	out := make([]rune, len(series))
	for i, v := range series {
		idx := int(v / peak * float64(len(bars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(bars) {
			idx = len(bars) - 1
		}
		out[i] = bars[idx]
	}
	return string(out)
}

// durationResultJSON is a single test's duration trend in JSON form.
type durationResultJSON struct {
	Package    string    `json:"package"`
	Test       string    `json:"test"`
	Latest     float64   `json:"latest"`
	First      float64   `json:"first"`
	TrendPct   float64   `json:"trend_pct"`
	FreshRuns  int       `json:"fresh_runs"`
	CachedRuns int       `json:"cached_runs"`
	Series     []float64 `json:"series"`
}

// durationOutputJSON wraps the per-test results with the overall summary.
type durationOutputJSON struct {
	Overall struct {
		AvgFirst float64 `json:"avg_first"`
		AvgLast  float64 `json:"avg_last"`
		TrendPct float64 `json:"trend_pct"`
		Tests    int     `json:"tests"`
	} `json:"overall"`
	Results []durationResultJSON `json:"results"`
}

// displayDurationResultsJSON renders the duration trend results as JSON to stdout.
func displayDurationResultsJSON(results []trend.DurationResult) error {
	out := durationOutputJSON{
		Results: make([]durationResultJSON, 0, len(results)),
	}

	overall := trend.OverallDuration(results)
	out.Overall.AvgFirst = overall.AvgFirst
	out.Overall.AvgLast = overall.AvgLast
	out.Overall.TrendPct = overall.TrendPct
	out.Overall.Tests = overall.Tests

	for _, r := range results {
		out.Results = append(out.Results, durationResultJSON{
			Package:    r.Reference.Package,
			Test:       r.Reference.TestName(true),
			Latest:     r.Latest,
			First:      r.First,
			TrendPct:   r.TrendPct,
			FreshRuns:  r.FreshRuns,
			CachedRuns: r.CachedRuns,
			Series:     r.Series,
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}
