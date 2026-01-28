package commands

import (
	"github.com/spf13/cobra"

	"github.com/anchore/clio"
)

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
	)

	return cmd
}
