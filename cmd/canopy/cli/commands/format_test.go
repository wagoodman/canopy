package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
)

func TestRunFormat_StoresUnderNamedSession(t *testing.T) {
	events := `{"Action":"run","Package":"example.com/p","Test":"TestA"}
{"Action":"pass","Package":"example.com/p","Test":"TestA","Elapsed":0}
{"Action":"pass","Package":"example.com/p","Elapsed":0}
`
	file := filepath.Join(t.TempDir(), "events.jsonl")
	require.NoError(t, os.WriteFile(file, []byte(events), 0o600))

	cfg := *defaultFormatOptions()
	cfg.Enabled = true
	cfg.Root = t.TempDir()
	cfg.Format.File = file
	cfg.Format.Session = "x"
	require.NoError(t, cfg.Format.PostLoad())

	require.NoError(t, runFormat(context.Background(), nil, cfg, false))

	// the named session is found (not re-created) and holds the stored run
	m, err := test.NewManager(test.Config{DBRoot: cfg.Root, SessionName: "x"})
	require.NoError(t, err)
	defer m.Close()

	info, err := m.CurrentSession()
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Equal(t, "x", info.Name)
	require.Len(t, info.Runs, 1)
}
