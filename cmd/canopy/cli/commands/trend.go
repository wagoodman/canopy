package commands

import (
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
	"github.com/wagoodman/canopy/cmd/canopy/internal/trend"

	"github.com/anchore/clio"
)

// colTest is the shared "Test" table header used across trend subcommands (goconst).
const colTest = "Test"

// printRollupf writes a trend subcommand's summary line to stderr in the faint style,
// keeping stdout as the clean report. Shared so every trend rollup looks the same.
func printRollupf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n%s\n", styleAux.Render(fmt.Sprintf(format, args...)))
}

// collectTrendRecords opens the store, builds a scope from the shared trend options, and
// collects historical records. The manager is closed before returning since the trend
// analyzers operate on the returned in-memory data, not the store.
func collectTrendRecords(store options.Store, t options.Trend) (map[gotest.Reference][]trend.Record, error) {
	mgr, err := test.NewManager(test.Config{
		DBRoot:    store.Root,
		Ephemeral: store.Ephemeral,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create test manager: %w", err)
	}
	defer mgr.Close()

	// resolve session IDs from the config (handles "last" keyword)
	sessionIDs, err := resolveSessionIDs(mgr, t.Sessions)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve session IDs: %w", err)
	}

	// only use the compiled regex if TestStr was actually set. this works around a
	// potential clio/fangs init quirk where Test may be non-nil but invalid.
	var testPattern *regexp.Regexp
	if t.TestStr != "" {
		testPattern = t.Test
	}

	scope := trend.Scope{
		Last:            t.Last,
		Window:          t.Window,
		SessionIDs:      sessionIDs,
		PackagePatterns: t.Specifiers,
		ExcludePatterns: t.ExcludePatterns,
		TestPattern:     testPattern,
	}

	records, err := trend.Collect(mgr, scope)
	if err != nil {
		return nil, fmt.Errorf("unable to collect trend records: %w", err)
	}
	return records, nil
}

// Trend creates the trend analysis command with subcommands for analyzing historical test data.
// Trend commands provide insights into test behavior patterns across multiple runs.
func Trend(app clio.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trend",
		Short: "analyze test trends and patterns from historical data",
		Long: `Analyze historical test data to identify patterns and trends.

The trend command provides insights into test behavior across multiple runs,
helping identify flaky tests, performance regressions, and other patterns.`,
	}

	cmd.AddCommand(
		TrendFlaky(app),
		TrendDuration(app),
		TrendFailures(app),
		TrendCount(app),
	)

	return cmd
}
