package commands

import (
	"context"
	"fmt"
	"github.com/anchore/clio"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
)

type rootConfig struct {
	*testCoreConfig `yaml:",inline" json:",inline" mapstructure:",squash"`
}

func defaultRootOptions() *rootConfig {
	c := rootConfig{
		testCoreConfig: defaultTestOptions(),
	}

	c.Test.Packages.Specifiers = []string{"./..."} // default to all project packages

	return &c
}

func Root(app clio.Application) *cobra.Command {
	opts := defaultRootOptions()

	var runErr error
	return app.SetupRootCommand(&cobra.Command{
		Use:   fmt.Sprintf("%s [SOURCE]", app.ID().Name),
		Short: "select and run go tests",
		Args:  cobra.NoArgs, // TODO: should be the same as -run ? or should be package subselection? (the latter, also have --run flag)
		//Example: // TODO
		PreRunE: func(_ *cobra.Command, _ []string) error {
			// get the final set of packages to use
			testPkgs, err := golist.SelectPackages(opts.Test.Specifiers, opts.Test.ExcludePatterns)
			if err != nil {
				return fmt.Errorf("unble to get test paths: %w", err)
			}
			if testPkgs.Size() == 0 {
				return fmt.Errorf("no packages selected to test (given %q)", opts.Test.Specifiers)
			}
			opts.Test.Runtime.Packages = testPkgs

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			defer func() {
				if err := opts.Test.Writers.Close(); err != nil {
					runErr = multierror.Append(runErr, err)
					log.WithFields("error", err).Error("unable to close format writers")
				}
			}()

			runErr = runRoot(cmd.Context(), app, *opts)

			return nil
		},
	}, opts)
}

func runRoot(_ context.Context, app clio.Application, cfg rootConfig) error {
	testDefs, err := gotest.FindDefinitions(cfg.Test.Runtime.Packages)
	if err != nil {
		return err
	}

	id := app.ID()

	ux := ui.NewSelectorUI(selector.Config{
		ID:    fmt.Sprintf("%s@%s", id.Name, id.Version),
		Debug: false,
	}, testDefs)

	type Stater interface {
		State() *clio.State
	}

	state := app.(Stater).State()

	if err = state.UI.Replace(ui.NewCollection(ux)); err != nil {
		return fmt.Errorf("unable to replace UI: %w", err)
	}

	refs := ux.Prompt()

	// TODO: run tests! get selection from model!
	fmt.Printf("Selected references: %d\n", len(refs))

	return nil
}
