package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/flaky"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/failure"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"

	"github.com/anchore/clio"
)

type flakyConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`
	options.Store  `yaml:"store" json:"store" mapstructure:"store"`
	options.Flaky  `yaml:"flaky" json:"flaky" mapstructure:"flaky"`
}

// TrendFlaky creates a command to detect and report flaky tests from historical test sessions.
// Flaky tests are identified by analyzing pass/fail patterns across multiple test runs.
func TrendFlaky(app clio.Application) *cobra.Command {
	store := options.DefaultStore()
	store.Enabled = true
	opts := &flakyConfig{
		Store: store,
		Flaky: options.DefaultFlaky(),
	}

	cmd := &cobra.Command{
		Use:   "flaky [GO-PKG-SPECIFIER...]",
		Short: "detect and report flaky tests from historical sessions",
		Long: `Analyze test runs to identify tests with inconsistent outcomes.

A test is considered flaky if it has both passed and failed across multiple runs.
The flaky score ranges from 0 (completely stable) to 1 (maximally flaky, 50% pass rate).

Examples:
  # List all flaky tests
  canopy trend flaky

  # Analyze only specific packages
  canopy trend flaky ./cmd/canopy/...

  # Exclude specific packages
  canopy trend flaky ./... --exclude '**/vendor/*'

  # Only analyze tests matching a pattern
  canopy trend flaky --test 'TestUser.*'

  # Analyze only specific sessions
  canopy trend flaky --session abc123 --session def456

  # Analyze only the most recent session
  canopy trend flaky --session last

  # Only show tests that are at least 25% flaky
  canopy trend flaky --threshold 0.25

  # Limit analysis to tests with at least 5 runs
  canopy trend flaky --min-runs 5

  # Only analyze runs from the last 7 days
  canopy trend flaky --window 168h

  # Combine scoping options
  canopy trend flaky ./internal/auth/... --test 'TestLogin.*' --window 168h`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Specifiers = args
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runFlaky(*opts)
		},
	}

	// facilitates grouping of flags into sections in help text
	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupCommand(cmd, opts)
}

func runFlaky(cfg flakyConfig) error {
	log.Info("analyzing test flakiness from historical sessions")

	mgr, err := test.NewManager(
		test.Config{
			DBRoot:    cfg.Root,
			Ephemeral: cfg.Ephemeral,
		},
	)
	if err != nil {
		return fmt.Errorf("unable to create test manager: %w", err)
	}
	defer mgr.Close()

	// resolve session IDs from the config (handles "last" keyword)
	sessionIDs, err := resolveSessionIDs(mgr, cfg.Sessions)
	if err != nil {
		return fmt.Errorf("unable to resolve session IDs: %w", err)
	}

	// only use the compiled regex if TestStr was actually set
	// this works around a potential initialization bug in clio/fangs where the
	// Test field may be initialized to a non-nil but invalid *regexp.Regexp
	var testPattern *regexp.Regexp
	if cfg.TestStr != "" {
		testPattern = cfg.Test
	}

	analyzer := flaky.NewAnalyzer(mgr, flaky.Config{
		Window:          cfg.Window,
		MinRuns:         cfg.MinRuns,
		Threshold:       cfg.Threshold,
		PackagePatterns: cfg.Specifiers,
		ExcludePatterns: cfg.ExcludePatterns,
		TestPattern:     testPattern,
		SessionIDs:      sessionIDs,
	})

	results, err := analyzer.AnalyzeAll()
	if err != nil {
		return fmt.Errorf("unable to analyze flaky tests: %w", err)
	}

	if len(results) == 0 {
		if options.FlakyOutputFormat(cfg.Output) == options.FlakyOutputJSON {
			fmt.Println("[]")
		} else {
			fmt.Println("No flaky tests detected.")
		}
		return nil
	}

	switch options.FlakyOutputFormat(cfg.Output) {
	case options.FlakyOutputJSON:
		return displayFlakyResultsJSON(results)
	default:
		displayFlakyResults(results)
	}
	return nil
}

// displayFlakyResults renders the flaky test analysis results to stdout.
func displayFlakyResults(results []flaky.Analysis) {
	// sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = false

	t.AppendHeader(table.Row{"Test", "Score", "Runs", "Pass", "Fail", "Skip", "Distinct Failures", "Last Flip"})

	for _, r := range results {
		testName := formatTestReference(r.Reference)
		score := fmt.Sprintf("%.0f%%", r.Score*100)
		lastFlip := ""
		if r.LastFlip != nil {
			lastFlip = formatRelativeTime(r.LastFlip.Time)
		}

		t.AppendRow(table.Row{
			testName,
			score,
			r.TotalRuns,
			r.PassCount,
			r.FailCount,
			r.SkipCount,
			len(r.FailureModes),
			lastFlip,
		})
	}

	fmt.Println(t.Render())

	// print summary
	flakyCount := 0
	for _, r := range results {
		if r.IsFlaky() {
			flakyCount++
		}
	}

	fmt.Printf("\nFound %d flaky test(s) out of %d analyzed.\n", flakyCount, len(results))
}

// formatTestReference formats a test reference for display.
func formatTestReference(ref interface{ String(bool) string }) string {
	s := ref.String(false)
	// truncate long package paths for readability
	parts := strings.Split(s, "/")
	if len(parts) > 4 {
		// keep first part, ellipsis, last 3 parts
		s = parts[0] + "/.../" + strings.Join(parts[len(parts)-3:], "/")
	}
	return s
}

// formatRelativeTime formats a time as a relative duration from now.
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// resolveSessionIDs converts session specifiers (UUIDs or keywords like "last") to actual UUIDs.
func resolveSessionIDs(mgr *test.Manager, specifiers []string) ([]uuid.UUID, error) {
	if len(specifiers) == 0 {
		return nil, nil
	}

	var ids []uuid.UUID
	for _, spec := range specifiers {
		switch strings.ToLower(spec) {
		case "last", "latest":
			sessions, err := mgr.ListSessions()
			if err != nil {
				return nil, fmt.Errorf("unable to list sessions: %w", err)
			}
			if len(sessions) == 0 {
				return nil, fmt.Errorf("no sessions found for 'last' specifier")
			}
			// sessions are returned sorted by most recent first
			ids = append(ids, sessions[0].UUID)
		default:
			id, err := uuid.Parse(spec)
			if err != nil {
				return nil, fmt.Errorf("invalid session ID %q: %w", spec, err)
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// JSON output types

// runInfoJSON represents a test run occurrence in JSON format.
type runInfoJSON struct {
	ID   string    `json:"id"`
	Time time.Time `json:"time"`
}

// notableRunJSON represents a run at a flip point in JSON format.
type notableRunJSON struct {
	ID          string    `json:"id"`
	Time        time.Time `json:"time"`
	State       string    `json:"state"`
	Fingerprint string    `json:"fingerprint,omitempty"`
}

// flakyResultJSON represents a single flaky test result in JSON format.
type flakyResultJSON struct {
	Package         string            `json:"package"`
	Test            string            `json:"test"`
	Score           float64           `json:"score"`
	PassRate        float64           `json:"pass_rate"`
	TotalRuns       int               `json:"total_runs"`
	PassCount       int               `json:"pass_count"`
	FailCount       int               `json:"fail_count"`
	SkipCount       int               `json:"skip_count"`
	LastFlip        *runInfoJSON      `json:"last_flip,omitempty"`
	NotableRuns     []notableRunJSON  `json:"notable_runs,omitempty"`
	FailureModes    []failureModeJSON `json:"failure_modes,omitempty"`
	DistinctFailure int               `json:"distinct_failures"`
}

// failureModeJSON represents a failure mode in JSON format.
type failureModeJSON struct {
	Fingerprint string        `json:"fingerprint"`
	Type        failure.Type  `json:"type"`
	Count       int           `json:"count"`
	Runs        []runInfoJSON `json:"runs"`
}

// displayFlakyResultsJSON renders the flaky test analysis results as JSON to stdout.
func displayFlakyResultsJSON(results []flaky.Analysis) error {
	// sort by score descending for consistency
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	output := make([]flakyResultJSON, 0, len(results))
	for _, r := range results {
		item := flakyResultJSON{
			Package:         r.Reference.Package,
			Test:            r.Reference.TestName(true),
			Score:           r.Score,
			PassRate:        r.PassRate(),
			TotalRuns:       r.TotalRuns,
			PassCount:       r.PassCount,
			FailCount:       r.FailCount,
			SkipCount:       r.SkipCount,
			DistinctFailure: len(r.FailureModes),
		}

		if r.LastFlip != nil {
			item.LastFlip = &runInfoJSON{
				ID:   r.LastFlip.ID.String(),
				Time: r.LastFlip.Time,
			}
		}

		// convert notable runs (flip points)
		for _, run := range r.NotableRuns {
			item.NotableRuns = append(item.NotableRuns, notableRunJSON{
				ID:          run.ID.String(),
				Time:        run.Time,
				State:       string(run.State),
				Fingerprint: run.Fingerprint,
			})
		}

		// convert failure modes with their runs
		for _, fm := range r.FailureModes {
			fmJSON := failureModeJSON{
				Fingerprint: fm.Fingerprint,
				Type:        fm.Type,
				Count:       fm.Count(),
			}
			for _, run := range fm.Runs {
				fmJSON.Runs = append(fmJSON.Runs, runInfoJSON{
					ID:   run.ID.String(),
					Time: run.Time,
				})
			}
			item.FailureModes = append(item.FailureModes, fmJSON)
		}

		output = append(output, item)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
