package commands

import (
	"github.com/spf13/cobra"

	"github.com/anchore/clio"
)

// session list [ID]   ... shows session IDs and run IDs (or just the run ids for a given session)
// session open ID     ... opens a specific session
// session open        ... opens the most recent session

// Session creates the session management command with subcommands for listing and opening test sessions.
// Sessions store test run history and results in a SQLite database for later inspection.
func Session(app clio.Application) *cobra.Command {
	db := &cobra.Command{
		Use:   "session",
		Short: "manage canopy test sessions",
	}

	db.AddCommand(
		SessionList(app),
		SessionOpen(app),
	)

	return db
}
