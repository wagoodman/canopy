package ide

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wagoodman/canopy/cmd/canopy/internal/env"
)

func TestNewZed(t *testing.T) {
	t.Run("zed found", func(t *testing.T) {
		mockLookPath := func(file string) (string, error) {
			return "/path/to/zed", nil
		}

		z, err := NewZed(mockLookPath)
		assert.NoError(t, err)
		assert.NotNil(t, z)
		assert.Equal(t, "/path/to/zed", z.binPath)
	})

	t.Run("zed not found", func(t *testing.T) {
		mockLookPath := func(file string) (string, error) {
			return "", errors.New("zed not found")
		}

		z, err := NewZed(mockLookPath)
		assert.Error(t, err)
		assert.Nil(t, z)
		assert.EqualError(t, err, "unable to find zed binary: zed not found")
	})
}

func TestZed_isActive(t *testing.T) {
	tests := []struct {
		name   string
		env    map[string]string
		active bool
	}{
		{
			name: "cfBundleIdentifier set correctly",
			env: map[string]string{
				"__CFBundleIdentifier": "dev.zed.Zed",
			},
			active: true,
		},
		{
			name: "zedTerm set correctly",
			env: map[string]string{
				"ZED_TERM": "true",
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
			envGetter := env.NewSnapshotEnvironmentGetter(tt.env)
			z := Zed{}

			result := z.isActive(envGetter)

			assert.Equal(t, tt.active, result)
		})
	}
}

func TestZed_OpenFileAtLineCommand(t *testing.T) {
	z := Zed{binPath: "/path/to/zed"}
	command := z.OpenFileAtLineCommand("/path/to/file.go", 42)

	assert.Equal(t, `/path/to/zed "/path/to/file.go:42:0"`, command)
}

func TestZed_FileAtLineURL(t *testing.T) {
	z := Zed{}
	url := z.FileAtLineURL("/path/to/file.go", 42)

	assert.Equal(t, "file:///path/to/file.go:42", url)
}
