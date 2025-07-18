package cli

import (
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/commands"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"

	"github.com/anchore/clio"
)

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

	root.AddCommand(
		clio.VersionCommand(id),
		clio.ConfigCommand(app, nil),
		commands.Test(app),
		commands.List(app),
		commands.Session(app),
		commands.Format(app),
	)

	return root
}
