package commands

import (
	"github.com/spf13/cobra"

	"github.com/anchore/clio"
)

// session list [ID]   ... shows session IDs and run IDs (or just the run ids for a given session)
// session open ID     ... opens a specific session
// session open        ... opens the most recent session

func Session(app clio.Application) *cobra.Command {
	db := &cobra.Command{
		Use:   "session",
		Short: "manage canopy test sessions",
	}

	db.AddCommand(
		SessionList(app),
		SessionOpen(app),
	)

	// Add alias for `open` command to the `session open` command
	db.AddCommand(&cobra.Command{
		Use:   "open",
		Short: "alias for `session open` command",
		RunE: func(cmd *cobra.Command, args []string) error {
			return SessionOpen(app).RunE(cmd, args)
		},
	})

	return db
}
