package ide

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGoland(t *testing.T) {
	t.Run("goland found", func(t *testing.T) {
		mockLookPath := func(file string) (string, error) {
			return "/path/to/goland", nil
		}

		g, err := NewGoland(mockLookPath)
		assert.NoError(t, err)
		assert.NotNil(t, g)
		assert.Equal(t, "/path/to/goland", g.binPath)
	})

	t.Run("goland not found", func(t *testing.T) {
		mockLookPath := func(file string) (string, error) {
			return "", errors.New("goland not found")
		}

		g, err := NewGoland(mockLookPath)
		assert.Error(t, err)
		assert.Nil(t, g)
		assert.EqualError(t, err, "unable to find goland binary: goland not found")
	})
}

func TestGoland_isActive(t *testing.T) {
	tests := []struct {
		name   string
		env    map[string]string
		active bool
	}{
		{
			name: "cfBundleIdentifier set correctly",
			env: map[string]string{
				"__CFBundleIdentifier": "com.jetbrains.goland",
			},
			active: true,
		},
		{
			name: "terminalEmulator set correctly",
			env: map[string]string{
				"TERMINAL_EMULATOR": "JetBrains-JediTerm",
			},
			active: true,
		},
		{
			name:   "no relevant environment variables",
			env:    map[string]string{},
			active: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envGetter := NewSnapshotEnvironmentGetter(tt.env)
			g := Goland{}

			result := g.isActive(envGetter)

			assert.Equal(t, tt.active, result)
		})
	}
}

func TestGoland_OpenFileAtLineCommand(t *testing.T) {
	g := Goland{binPath: "/path/to/goland"}
	command := g.OpenFileAtLineCommand("/path/to/file.go", 42)

	assert.Equal(t, "/path/to/goland --line 42 /path/to/file.go", command)
}

func TestGoland_FileAtLineURL(t *testing.T) {
	g := Goland{}
	url := g.FileAtLineURL("/path/to/file.go", 42)

	assert.Equal(t, "file:///path/to/file.go:42", url)
}
