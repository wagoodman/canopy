package cienv

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestParseToggle(t *testing.T) {
	tests := []struct {
		input    string
		expected Toggle
		wantErr  bool
	}{
		// Auto values
		{"", ToggleAuto, false},
		{"auto", ToggleAuto, false},
		{"AUTO", ToggleAuto, false},
		{"  auto  ", ToggleAuto, false},

		// On values
		{"on", ToggleOn, false},
		{"ON", ToggleOn, false},
		{"true", ToggleOn, false},
		{"TRUE", ToggleOn, false},
		{"enabled", ToggleOn, false},
		{"always", ToggleOn, false},
		{"yes", ToggleOn, false},
		{"1", ToggleOn, false},

		// Off values
		{"off", ToggleOff, false},
		{"OFF", ToggleOff, false},
		{"false", ToggleOff, false},
		{"FALSE", ToggleOff, false},
		{"disabled", ToggleOff, false},
		{"never", ToggleOff, false},
		{"no", ToggleOff, false},
		{"0", ToggleOff, false},

		// Invalid values
		{"invalid", ToggleAuto, true},
		{"maybe", ToggleAuto, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseToggle(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

func TestToggle_String(t *testing.T) {
	assert.Equal(t, "auto", ToggleAuto.String())
	assert.Equal(t, "on", ToggleOn.String())
	assert.Equal(t, "off", ToggleOff.String())
}

func TestToggle_Is(t *testing.T) {
	assert.True(t, ToggleAuto.IsAuto())
	assert.False(t, ToggleAuto.IsOn())
	assert.False(t, ToggleAuto.IsOff())

	assert.False(t, ToggleOn.IsAuto())
	assert.True(t, ToggleOn.IsOn())
	assert.False(t, ToggleOn.IsOff())

	assert.False(t, ToggleOff.IsAuto())
	assert.False(t, ToggleOff.IsOn())
	assert.True(t, ToggleOff.IsOff())
}

func TestToggle_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected Toggle
	}{
		{"boolean true", "enabled: true", ToggleOn},
		{"boolean false", "enabled: false", ToggleOff},
		{"string auto", "enabled: auto", ToggleAuto},
		{"string on", "enabled: on", ToggleOn},
		{"string off", "enabled: off", ToggleOff},
		{"string enabled", "enabled: enabled", ToggleOn},
		{"string disabled", "enabled: disabled", ToggleOff},
		{"empty string", "enabled: \"\"", ToggleAuto},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg struct {
				Enabled Toggle `yaml:"enabled"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.Enabled)
		})
	}
}

func TestToggle_MarshalYAML(t *testing.T) {
	tests := []struct {
		toggle   Toggle
		expected string
	}{
		{ToggleAuto, "auto"},
		{ToggleOn, "on"},
		{ToggleOff, "off"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			cfg := struct {
				Enabled Toggle `yaml:"enabled"`
			}{Enabled: tt.toggle}

			data, err := yaml.Marshal(&cfg)
			require.NoError(t, err)
			assert.Contains(t, string(data), tt.expected)
		})
	}
}

func TestToggle_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected Toggle
	}{
		{"boolean true", `{"enabled": true}`, ToggleOn},
		{"boolean false", `{"enabled": false}`, ToggleOff},
		{"string auto", `{"enabled": "auto"}`, ToggleAuto},
		{"string on", `{"enabled": "on"}`, ToggleOn},
		{"string off", `{"enabled": "off"}`, ToggleOff},
		{"empty string", `{"enabled": ""}`, ToggleAuto},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg struct {
				Enabled Toggle `json:"enabled"`
			}
			err := json.Unmarshal([]byte(tt.json), &cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, cfg.Enabled)
		})
	}
}

func TestToggle_MarshalJSON(t *testing.T) {
	tests := []struct {
		toggle   Toggle
		expected string
	}{
		{ToggleAuto, `"auto"`},
		{ToggleOn, `"on"`},
		{ToggleOff, `"off"`},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			cfg := struct {
				Enabled Toggle `json:"enabled"`
			}{Enabled: tt.toggle}

			data, err := json.Marshal(&cfg)
			require.NoError(t, err)
			assert.Contains(t, string(data), tt.expected)
		})
	}
}
