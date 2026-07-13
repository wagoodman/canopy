package commands

import (
	"github.com/spf13/cobra"

	"github.com/anchore/clio"
)

// List creates the list command with subcommands for listing test definitions and historical runs.
func List(app clio.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list test definitions, historical runs, or sessions",
	}

	cmd.AddCommand(
		ListDefs(app),
		ListRuns(app),
		ListSessions(app),
	)

	return cmd
}
