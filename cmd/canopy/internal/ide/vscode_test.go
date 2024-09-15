package ide

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewVSCode(t *testing.T) {
	t.Run("code found", func(t *testing.T) {
		mockLookPath := func(file string) (string, error) {
			return "/path/to/code", nil
		}

		v, err := NewVSCode(mockLookPath)
		assert.NoError(t, err)
		assert.NotNil(t, v)
		assert.Equal(t, "/path/to/code", v.binPath)
	})

	t.Run("code not found", func(t *testing.T) {
		mockLookPath := func(file string) (string, error) {
			return "", errors.New("code not found")
		}

		v, err := NewVSCode(mockLookPath)
		assert.Error(t, err)
		assert.Nil(t, v)
		assert.EqualError(t, err, "unable to find code binary: code not found")
	})
}

func TestVSCode_isActive(t *testing.T) {
	tests := []struct {
		name   string
		env    map[string]string
		active bool
	}{
		{
			name: "termProgram set correctly",
			env: map[string]string{
				"TERM_PROGRAM": "vscode",
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
			v := VSCode{}

			result := v.isActive(envGetter)

			assert.Equal(t, tt.active, result)
		})
	}
}

func TestVSCode_OpenFileAtLineCommand(t *testing.T) {
	v := VSCode{binPath: "/path/to/code"}
	command := v.OpenFileAtLineCommand("/path/to/file.go", 42)

	assert.Equal(t, `/path/to/code --goto "/path/to/file.go:42"`, command)
}

func TestVSCode_FileAtLineURL(t *testing.T) {
	v := VSCode{}
	url := v.FileAtLineURL("/path/to/file.go", 42)

	assert.Equal(t, "file:///path/to/file.go:42", url)
}
