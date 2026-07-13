package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"

	"github.com/anchore/clio"
	"github.com/anchore/go-logger/adapter/discard"
)

// type sessionOpenConfig struct {
//	options.Config `yaml:",inline" mapstructure:",squash"`
//	options.Store  `yaml:"store" json:"store" mapstructure:"store"`
//	SessionID      string `yaml:"session-id" json:"session-id" mapstructure:"session-id"`
//}

// Open creates a command to open an interactive UI session from previously stored test results.
// NAME selects the session (find-or-create); it defaults to the current git branch.
func Open(app clio.Application) *cobra.Command {
	storeCfg := options.DefaultStore()
	storeCfg.Enabled = true
	opts := &sessionListConfig{
		Store: storeCfg,
	}

	var ux *ui.StudioUI

	// rawName holds the requested session name; bare invocation defaults to the current branch
	rawName := defaultSessionName

	cmd := &cobra.Command{
		Use:   "open [NAME]",
		Short: "open an interactive session from existing test results",
		Long: "open an interactive session by name (find-or-create).\n\n" +
			"NAME defaults to @branch. resolvers: @branch (current git branch), " +
			"@module (go module path), @worktree (worktree root basename).",
		Args: func(_ *cobra.Command, args []string) error {
			if err := cobra.MaximumNArgs(1)(nil, args); err != nil {
				return err
			}
			if len(args) == 1 {
				rawName = args[0]
			}
			return nil
		},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			var err error
			ux, err = setupSessionOpen(app, opts.Store, resolveSessionName(rawName))
			return err
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSessionOpen(ux)
		},
	}

	// facilitates grouping of flags into sections in help text
	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupCommand(cmd, opts)
}

func runSessionOpen(ux *ui.StudioUI) error {
	ux.Wait()
	return nil
}

// setupSessionOpen loads the named test session (find-or-create) and initializes the Studio UI for interactive browsing.
func setupSessionOpen(app clio.Application, storeConfig options.Store, sessionName string) (*ui.StudioUI, error) {
	// get the session
	s, err := test.NewManager(
		test.Config{
			DBRoot:      storeConfig.Root,
			Ephemeral:   storeConfig.Ephemeral,
			SessionName: sessionName,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load test session: %w", err)
	}

	sessionInfo, err := s.CurrentSession()
	if err != nil {
		return nil, fmt.Errorf("unable to get current test session: %w", err)
	}
	if sessionInfo == nil {
		return nil, fmt.Errorf("no test session found")
	}

	// set the UI

	// TODO: buffer elsewhere?
	log.Set(discard.New())

	type Stater interface {
		State() *clio.State
	}

	state := app.(Stater).State()

	id := app.ID()

	ux := ui.NewStudioUI(
		studio.Config{
			ID:            fmt.Sprintf("%s@%s", id.Name, id.Version),
			RunStore:      s,
			RunController: s,
			SessionInfo:   *sessionInfo,
			Debug:         true, // TODO make this accessible via env var or similar
		},
	)

	return ux, state.UI.Replace(ux)
}
