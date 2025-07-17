package commands

import (
	"fmt"
	"github.com/spf13/cobra"

	"github.com/anchore/clio"
)

func Root(app clio.Application, testCmd *cobra.Command) *cobra.Command {
	opts := defaultTestOptions()

	return app.SetupRootCommand(&cobra.Command{
		Use:     fmt.Sprintf("%s [SOURCE]", app.ID().Name),
		Short:   testCmd.Short,
		Long:    testCmd.Long,
		Args:    testCmd.Args,
		Example: testCmd.Example,
		PreRunE: testCmd.PreRunE,
		RunE:    testCmd.RunE,
	}, opts)
}
