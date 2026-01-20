package env

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOSEnvironmentGetter(t *testing.T) {
	tests := []struct {
		name      string
		setupEnv  map[string]string
		getKey    string
		expectVal string
		expectOk  bool
	}{
		{
			name:      "get existing env var",
			setupEnv:  map[string]string{"TEST_ENV": "test_value"},
			getKey:    "TEST_ENV",
			expectVal: "test_value",
			expectOk:  true,
		},
		{
			name:      "get non-existent env var",
			setupEnv:  map[string]string{},
			getKey:    "NON_EXISTENT_ENV",
			expectVal: "",
			expectOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.setupEnv {
				require.NoError(t, os.Setenv(k, v))
				t.Cleanup(func() { require.NoError(t, os.Unsetenv(k)) })
			}

			envGetter := &OSEnvironmentGetter{}
			val := envGetter.Getenv(tt.getKey)
			assert.Equal(t, tt.expectVal, val)

			val, ok := envGetter.LookupEnv(tt.getKey)
			assert.Equal(t, tt.expectOk, ok)
			assert.Equal(t, tt.expectVal, val)
		})
	}
}

func TestSnapshotEnvironmentGetter(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		getKey    string
		expectVal string
		expectOk  bool
	}{
		{
			name:      "get existing env var",
			env:       map[string]string{"TEST_ENV": "test_value"},
			getKey:    "TEST_ENV",
			expectVal: "test_value",
			expectOk:  true,
		},
		{
			name:      "get non-existent env var",
			env:       map[string]string{},
			getKey:    "NON_EXISTENT_ENV",
			expectVal: "",
			expectOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envGetter := NewSnapshotEnvironmentGetter(tt.env)

			val := envGetter.Getenv(tt.getKey)
			assert.Equal(t, tt.expectVal, val)

			val, ok := envGetter.LookupEnv(tt.getKey)
			assert.Equal(t, tt.expectOk, ok)
			assert.Equal(t, tt.expectVal, val)
		})
	}
}

func TestNewSnapshotEnvironmentGetterFromOSEnv(t *testing.T) {
	require.NoError(t, os.Setenv("TEST_ENV", "test_value"))
	t.Cleanup(func() { require.NoError(t, os.Unsetenv("TEST_ENV")) })

	envGetter := NewSnapshotEnvironmentGetterFromOSEnv()
	value, ok := envGetter.LookupEnv("TEST_ENV")

	assert.True(t, ok)
	assert.Equal(t, "test_value", value)
}

func TestTruthy(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		// truthy values
		{"1", true},
		{"t", true},
		{"T", true},
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"yes", true},
		{"Yes", true},
		{"YES", true},
		{"y", true},
		{"Y", true},
		{"on", true},
		{"On", true},
		{"ON", true},
		// with whitespace
		{"  true  ", true},
		{"\ttrue\n", true},
		// falsy values
		{"", false},
		{"0", false},
		{"f", false},
		{"false", false},
		{"False", false},
		{"no", false},
		{"No", false},
		{"n", false},
		{"off", false},
		{"random", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Truthy(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
