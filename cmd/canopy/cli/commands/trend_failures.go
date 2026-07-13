package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/trend"

	"github.com/anchore/clio"
)

type failuresConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`
	options.Store  `yaml:"store" json:"store" mapstructure:"store"`
	options.Trend  `yaml:"trend" json:"trend" mapstructure:"trend"`
}

// TrendFailures creates a command to report failure-rate trends from historical test sessions.
// It surfaces the tests failing most often and whether they are trending toward regression or recovery.
func TrendFailures(app clio.Application) *cobra.Command {
	store := options.DefaultStore()
	store.Enabled = true
	opts := &failuresConfig{
		Store: store,
		Trend: options.DefaultTrend(),
	}

	cmd := &cobra.Command{
		Use:   "failures [GO-PKG-SPECIFIER...]",
		Short: "report failure-rate trends from historical sessions",
		Long: `Analyze test runs to surface failure rates and how they are trending.

Tests are ranked by failure rate (fails over pass+fail, skips excluded). The trend
column flags whether failures are clustering in the later half of the window ("up",
a regression signal) or thinning out ("down", fixes landing).

Examples:
  # List tests by failure rate
  canopy trend failures

  # Analyze only specific packages
  canopy trend failures ./cmd/canopy/...

  # Exclude specific packages
  canopy trend failures ./... --exclude '**/vendor/*'

  # Only analyze tests matching a pattern
  canopy trend failures --test 'TestUser.*'

  # Analyze only specific sessions
  canopy trend failures --session abc123 --session def456

  # Analyze only the most recent session
  canopy trend failures --session last

  # Only analyze runs from the last 7 days
  canopy trend failures --window 168h`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Specifiers = args
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runFailures(*opts)
		},
	}

	// facilitates grouping of flags into sections in help text
	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupCommand(cmd, opts)
}

func runFailures(cfg failuresConfig) error {
	log.Info("analyzing test failure trends from historical sessions")

	records, err := collectTrendRecords(cfg.Store, cfg.Trend)
	if err != nil {
		return err
	}

	results := trend.FailureRates(records)

	if len(results) == 0 {
		if options.TrendOutputFormat(cfg.Output) == options.TrendOutputJSON {
			fmt.Println("[]")
		} else {
			fmt.Println("No failing tests detected.")
		}
		return nil
	}

	switch options.TrendOutputFormat(cfg.Output) {
	case options.TrendOutputJSON:
		return displayFailureResultsJSON(results)
	default:
		displayFailureResults(results)
	}
	return nil
}

// failure trend glyphs: ↑ worsening (red), ↓ improving (green), → steady (faint).
var (
	trendArrowUp   = styleChange.Render("↑")                                          // regression
	trendArrowDown = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("↓") // recovery (green)
	trendArrowFlat = styleAux.Render("→")                                             // steady
)

// trendSymbol renders a failure trend as a directional arrow.
func trendSymbol(t trend.FailureTrend) string {
	switch t {
	case trend.FailureTrendUp:
		return trendArrowUp
	case trend.FailureTrendDown:
		return trendArrowDown
	default:
		return trendArrowFlat
	}
}

// failFraction renders fails over attempts (skips excluded) with the rate, e.g. "6/10 (60%)".
// the percentage is derived/aux info, so it's rendered faint.
func failFraction(r trend.FailureResult) string {
	attempts := r.PassCount + r.FailCount
	var pct float64
	if attempts > 0 {
		pct = float64(r.FailCount) / float64(attempts) * 100
	}
	return fmt.Sprintf("%d/%d %s", r.FailCount, attempts, styleAux.Render(fmt.Sprintf("(%.0f%%)", pct)))
}

// displayFailureResults renders the failure-rate analysis as a table to stdout.
func displayFailureResults(results []trend.FailureResult) {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = false

	t.AppendHeader(table.Row{colTest, "Failures", "Trend"})

	var totalPass, totalFail int
	for _, r := range results {
		totalPass += r.PassCount
		totalFail += r.FailCount

		t.AppendRow(table.Row{
			r.Reference.String(false),
			failFraction(r),
			trendSymbol(r.Trend),
		})
	}

	fmt.Println(t.Render())

	// overall failure rate across all in-scope pass/fail observations
	var overall float64
	if denom := totalPass + totalFail; denom > 0 {
		overall = float64(totalFail) / float64(denom)
	}
	printRollupf("%d failing test(s); overall failure rate %.0f%% (%d/%d)",
		len(results), overall*100, totalFail, totalPass+totalFail)
}

// failureResultJSON represents a single failure-rate result in JSON format.
type failureResultJSON struct {
	Package   string  `json:"package"`
	Test      string  `json:"test"`
	Runs      int     `json:"runs"`
	PassCount int     `json:"pass_count"`
	FailCount int     `json:"fail_count"`
	SkipCount int     `json:"skip_count"`
	FailRate  float64 `json:"fail_rate"`
	Trend     string  `json:"trend"`
	Series    []bool  `json:"series"`
}

// displayFailureResultsJSON renders the failure-rate analysis as JSON to stdout.
func displayFailureResultsJSON(results []trend.FailureResult) error {
	output := make([]failureResultJSON, 0, len(results))
	for _, r := range results {
		output = append(output, failureResultJSON{
			Package:   r.Reference.Package,
			Test:      r.Reference.TestName(true),
			Runs:      r.Runs,
			PassCount: r.PassCount,
			FailCount: r.FailCount,
			SkipCount: r.SkipCount,
			FailRate:  r.FailRate,
			Trend:     string(r.Trend),
			Series:    r.Series,
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
