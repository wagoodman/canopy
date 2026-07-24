package commands

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
)

// shared output-format and detail-unit identifiers used across list/coverage commands.
const (
	formatJSON     = "json"
	formatTable    = "table"
	formatID       = "id"
	formatPackage  = "package"
	formatFunction = "function"
)

// writeJSON encodes the given value as indented JSON.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)

	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("unable to encode as JSON: %w", err)
	}
	return nil
}

// newTable returns a table writer with the borderless style used by all list commands.
func newTable() table.Writer {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = false
	return t
}
