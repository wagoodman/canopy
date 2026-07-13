// Package cli provides the command-line interface initialization and configuration for canopy.
package cli

import (
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/commands"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"

	"github.com/anchore/clio"
)

// New creates and configures the root cobra command with all subcommands and global configuration.
// It initializes the clio application framework with configuration file support, logging flags,
// and the event bus for coordinating test execution and UI updates.
func New(id clio.Identification) *cobra.Command {
	clioCfg := clio.NewSetupConfig(id).
		WithGlobalConfigFlag().   // add persistent -c <path> for reading an application config from
		WithGlobalLoggingFlags(). // add persistent -v and -q flags tied to the logging config
		WithUI(ui.TestNoUI()).
		WithInitializers(
			func(state *clio.State) error {
				bus.Set(state.Bus)
				log.Set(state.Logger)
				return nil
			},
		)

	app := clio.New(*clioCfg)

	root := commands.Root(app)

	app.AddFlags(root.PersistentFlags())

	openCmd := commands.SessionOpen(app)
	openCmd.Use = "open [SESSION-ID]"
	openCmd.Short = "open an interactive session from existing test results (alias for `session open` command)"

	root.AddCommand(
		clio.VersionCommand(id),
		clio.ConfigCommand(app, nil),
		commands.Test(app),
		commands.List(app),
		commands.Session(app),
		commands.Format(app),
		commands.Show(app),
		commands.Coverage(app),
		commands.Trend(app),
		commands.Affected(app),
		commands.Triage(app),
		commands.DB(app),

		// Add alias for `open` command to the `session open` command
		openCmd,
	)

	return root
}
