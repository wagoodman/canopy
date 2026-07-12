package options

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatPostLoad_FileDisallowed(t *testing.T) {
	// a file-disallowed format targeted at a file must be rejected (previously the
	// "name=path" string was compared against bare names and never matched)
	o := Format{
		Outputs:          []string{"log=" + filepath.Join(t.TempDir(), "out.txt")},
		AllowMultiple:    true,
		AllowableFormats: []string{"go", "json", "log"},
		FileDisallowed:   []string{"log"},
	}

	err := o.PostLoad()
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be written to a file")
}

func TestFormatPostLoad_AllowedFormatToFile(t *testing.T) {
	// a non-disallowed format targeted at a file must still be permitted
	path := filepath.Join(t.TempDir(), "out.json")
	o := Format{
		Outputs:          []string{"json=" + path},
		AllowMultiple:    true,
		AllowableFormats: []string{"go", "json", "log"},
		FileDisallowed:   []string{"log"},
	}

	err := o.PostLoad()
	require.NoError(t, err)
	require.NoError(t, o.Writers.Close())
}
