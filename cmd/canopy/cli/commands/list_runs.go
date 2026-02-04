package commands

import (
	"bytes"
	"encoding/json"
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

var _ fangs.FlagAdder = (*listRunsConfig)(nil)

type listRunsConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`
	options.Store  `yaml:"store" json:"store" mapstructure:"store"`

	// Last limits the output to the N most recent runs.
	Last int `yaml:"last" json:"last" mapstructure:"last"`
	// Output controls the output format: "table" (default) or "json".
	Output string `yaml:"output" json:"output" mapstructure:"output"`
}

func (o *listRunsConfig) AddFlags(flags fangs.FlagSet) {
	flags.IntVarP(&o.Last, "last", "", "show only the last N runs")
	flags.StringVarP(&o.Output, "output", "o", "output format (table, json)")
}

// ListRuns creates a command to display historical test run IDs and metadata.
// Run IDs are written to stdout for scriptability, while metadata is written to stderr.
func ListRuns(app clio.Application) *cobra.Command {
	store := options.DefaultStore()
	store.Enabled = true
	store.HideEnabledFlag = true

	opts := &listRunsConfig{
		Store:  store,
		Output: "table",
	}

	cmd := &cobra.Command{
		Use:   "runs",
		Short: "list historical test run IDs",
		Long: `List historical test run IDs from the database.

Run IDs are written to stdout for scriptability, metadata goes to stderr.

Examples:
  canopy list runs                    # list recent runs
  canopy list runs --last 20          # last 20 runs
  canopy list runs -o json            # JSON output

  # pipe run IDs to another command
  canopy list runs | head -1 | xargs canopy show`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runListRuns(*opts)
		},
	}

	// facilitates grouping of flags into sections in help text
	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupCommand(cmd, opts)
}

// runListEntry is a flattened representation of a run for display and serialization.
type runListEntry struct {
	RunID     string     `json:"run_id"`
	SessionID string     `json:"session_id"`
	Started   time.Time  `json:"started"`
	Ended     *time.Time `json:"ended,omitempty"`
	Elapsed   string     `json:"elapsed,omitempty"`
}

func runListRuns(cfg listRunsConfig) error {
	log.Info("listing test runs")

	m, err := test.NewManager(
		test.Config{
			DBRoot:    cfg.Root,
			Ephemeral: cfg.Ephemeral,
		},
	)
	if err != nil {
		return fmt.Errorf("unable to create test manager: %w", err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			log.WithFields("error", err).Error("unable to close test manager")
		}
	}()

	sessions, err := m.ListSessions()
	if err != nil {
		return fmt.Errorf("unable to list sessions: %w", err)
	}

	// flatten all runs from all sessions
	entries := collectRunEntries(sessions)

	// sort by start time (most recent first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Started.After(entries[j].Started)
	})

	// apply --last filter
	if cfg.Last > 0 && cfg.Last < len(entries) {
		entries = entries[:cfg.Last]
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "no test runs found")
		return nil
	}

	switch strings.ToLower(cfg.Output) {
	case "json":
		return writeRunsJSON(os.Stdout, entries)
	case "table", "":
		return writeRunsTable(os.Stdout, os.Stderr, entries)
	default:
		return fmt.Errorf("unknown output format: %s", cfg.Output)
	}
}

func collectRunEntries(sessions []test.SessionInfo) []runListEntry {
	var entries []runListEntry
	for _, session := range sessions {
		for _, run := range session.Runs {
			entry := runListEntry{
				RunID:     run.UUID.String(),
				SessionID: session.UUID.String(),
				Started:   run.Started,
				Ended:     run.Ended,
			}
			if run.Ended != nil {
				entry.Elapsed = run.Ended.Sub(run.Started).Round(time.Millisecond).String()
			}
			entries = append(entries, entry)
		}
	}
	return entries
}

// writeRunsTable writes run IDs to stdout (one per line) and a metadata table to stderr.
func writeRunsTable(stdout, stderr io.Writer, entries []runListEntry) error {
	// write IDs to stdout for scriptability
	for _, entry := range entries {
		fmt.Fprintln(stdout, entry.RunID)
	}

	// write metadata table to stderr
	t := table.NewWriter()
	t.SetOutputMirror(stderr)
	t.SetStyle(table.StyleLight)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = false

	t.AppendHeader(table.Row{"Run ID", "Session", "Started", "Elapsed"})

	for _, entry := range entries {
		t.AppendRow(table.Row{
			entry.RunID,
			entry.SessionID[:8], // abbreviated session ID
			fmtTime(&entry.Started),
			entry.Elapsed,
		})
	}

	t.Render()

	return nil
}

// writeRunsJSON writes all run entries as JSON to stdout.
func writeRunsJSON(w io.Writer, entries []runListEntry) error {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)

	if err := enc.Encode(entries); err != nil {
		return fmt.Errorf("unable to encode runs as JSON: %w", err)
	}

	_, err := w.Write(buf.Bytes())
	return err
}
