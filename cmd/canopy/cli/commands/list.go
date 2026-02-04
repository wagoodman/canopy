package commands

import (
	"github.com/spf13/cobra"

	"github.com/anchore/clio"
)

// List creates the list command with subcommands for listing test definitions and historical runs.
func List(app clio.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list test definitions or historical test runs",
	}

	cmd.AddCommand(
		ListDefs(app),
		ListRuns(app),
	)

	return cmd
}
