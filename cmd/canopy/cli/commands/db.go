package commands

import (
	"github.com/spf13/cobra"

	"github.com/anchore/clio"
)

// DB creates the database management command with subcommands for maintenance operations.
func DB(app clio.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "manage the canopy database",
	}

	cmd.AddCommand(
		DBPrune(app),
	)

	return cmd
}
