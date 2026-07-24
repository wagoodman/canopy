package commands

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"

	"github.com/anchore/clio"
	"github.com/anchore/fangs"
)

var _ fangs.FlagAdder = (*sessionListConfig)(nil)

type sessionListConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`
	options.Store  `yaml:"store" json:"store" mapstructure:"store"`
	// SessionID specifies which session to list runs for (if empty, lists all sessions).
	SessionID string `yaml:"session-id" json:"session-id" mapstructure:"session-id"`
	// Output controls the output format: "table" (default), "id", or "json".
	Output string `yaml:"output" json:"output" mapstructure:"output"`
}

func (o *sessionListConfig) AddFlags(flags fangs.FlagSet) {
	flags.StringVarP(&o.Output, "output", "o", "output format (table, id, json)")
}

// ListSessions creates a command to display all test sessions and their associated run information.
// Sessions are shown with their name, UUID, start time, duration, and number of test runs.
func ListSessions(app clio.Application) *cobra.Command {
	store := options.DefaultStore()
	store.Enabled = true
	opts := &sessionListConfig{
		Store:  store,
		Output: formatTable,
	}

	cmd := &cobra.Command{
		Use:   "sessions [SESSION-ID]",
		Short: "list sessions and the runs grouped under each",
		Args: func(_ *cobra.Command, args []string) error {
			if err := cobra.MaximumNArgs(1)(nil, args); err != nil {
				return err
			}
			if len(args) == 1 {
				opts.SessionID = args[0]
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runSessionList(*opts)
		},
	}

	// facilitates grouping of flags into sections in help text
	xflagset.BindCobraHelpFromOpts(cmd, opts)

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
	defer func() {
		if err := s.Close(); err != nil {
			log.WithFields("error", err).Error("unable to close test session")
		}
	}()

	sessions, err := s.ListSessions()
	if err != nil {
		return fmt.Errorf("unable to list test sessions: %w", err)
	}

	// if a specific session id was given, narrow the listing to just that session
	if cfg.SessionID != "" {
		filtered := sessions[:0]
		for i := range sessions {
			if sessions[i].UUID.String() == cfg.SessionID {
				filtered = append(filtered, sessions[i])
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("session %q not found", cfg.SessionID)
		}
		sessions = filtered
	}

	entries := collectSessionEntries(sessions)

	switch strings.ToLower(cfg.Output) {
	case formatJSON:
		return writeJSON(os.Stdout, entries)
	case formatID:
		writeSessionIDs(os.Stdout, entries)
		return nil
	case formatTable, "":
		writeSessionsTable(os.Stdout, entries)
		return nil
	default:
		return fmt.Errorf("unknown output format: %s", cfg.Output)
	}
}

// sessionListEntry is a flattened representation of a session for display and serialization.
type sessionListEntry struct {
	SessionID string     `json:"session_id"`
	Name      string     `json:"name,omitempty"`
	Started   time.Time  `json:"started"`
	Ended     *time.Time `json:"ended,omitempty"`
	Elapsed   string     `json:"elapsed,omitempty"`
	Runs      int        `json:"runs"`
}

func collectSessionEntries(sessions []test.SessionInfo) []sessionListEntry {
	var entries []sessionListEntry
	for i := range sessions {
		session := sessions[i]
		entries = append(entries, sessionListEntry{
			SessionID: session.UUID.String(),
			Name:      session.Name,
			Started:   session.Started,
			Ended:     session.Ended,
			Elapsed:   fmtElapsed(session.Started, session.Ended),
			Runs:      len(session.Runs),
		})
	}

	// most recent first
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Started.After(entries[j].Started)
	})

	return entries
}

// writeSessionIDs writes session IDs one per line for scriptability.
func writeSessionIDs(w io.Writer, entries []sessionListEntry) {
	for _, entry := range entries {
		fmt.Fprintln(w, entry.SessionID)
	}
}

func writeSessionsTable(w io.Writer, entries []sessionListEntry) {
	t := newTable()
	t.SetOutputMirror(w)

	t.AppendHeader(table.Row{"Name", "Session", "Started", "Elapsed", "Test Runs"})
	for _, entry := range entries {
		t.AppendRow(table.Row{
			entry.Name,
			entry.SessionID,
			fmtTime(&entry.Started),
			entry.Elapsed,
			entry.Runs,
		})
	}

	t.Render()
}

// fmtTime formats a time pointer as a string in "YYYY-MM-DD HH:MM:SS" format.
func fmtTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// fmtElapsed calculates and formats the duration between a start time and optional end time.
func fmtElapsed(started time.Time, ended *time.Time) string {
	if ended == nil {
		return ""
	}
	return ended.Sub(started).String()
}
