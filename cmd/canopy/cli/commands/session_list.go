package commands

import (
	"fmt"
	"sort"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"

	"github.com/anchore/clio"
)

type sessionListConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`
	options.Store  `yaml:"store" json:"store" mapstructure:"store"`
	SessionID      string `yaml:"session-id" json:"session-id" mapstructure:"session-id"`
}

func SessionList(app clio.Application) *cobra.Command {
	store := options.DefaultStore()
	store.Enabled = true
	opts := &sessionListConfig{
		Store: store,
	}

	cmd := &cobra.Command{
		Use:   "list [SESSION-ID]",
		Short: "list sessions and runs related to each session",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSessionList(*opts)
		},
	}

	return app.SetupCommand(cmd, opts)
}

func runSessionList(cfg sessionListConfig) error {
	log.WithFields("id", cfg.SessionID).Info("listing test sessions")

	s, err := test.NewManager(
		test.Config{
			DBRoot:    cfg.Root,
			Ephemeral: cfg.Ephemeral,
		},
	)
	if err != nil {
		return fmt.Errorf("unable to create test session: %w", err)
	}

	sessions, err := s.ListSessions()
	if err != nil {
		return fmt.Errorf("unable to list test sessions: %w", err)
	}

	var rows []table.Row
	for i := range sessions {
		session := sessions[i]
		rows = append(rows, table.Row{
			session.UUID.String(),
			fmtTime(&session.Started),
			fmtElapsed(session.Started, session.Ended),
			len(session.Runs),
		})
	}

	// sort rows by start time
	sort.Slice(rows, func(i, j int) bool {
		return rows[i][1].(string) > rows[j][1].(string)
	})

	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = false

	t.AppendHeader(table.Row{"Session", "Started", "Elapsed", "Test Runs"})
	t.AppendRows(rows)

	fmt.Println(t.Render())

	return nil
}

func fmtTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func fmtElapsed(started time.Time, ended *time.Time) string {
	if ended == nil {
		return ""
	}
	return ended.Sub(started).String()
}
