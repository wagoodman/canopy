package cienv

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Toggle represents a tri-state setting that can be auto-detected, explicitly on, or explicitly off.
type Toggle string

const (
	// ToggleAuto indicates the setting should be auto-detected from the environment.
	ToggleAuto Toggle = ""
	// ToggleOn indicates the setting is explicitly enabled.
	ToggleOn Toggle = "on"
	// ToggleOff indicates the setting is explicitly disabled.
	ToggleOff Toggle = "off"
)

// String returns the string representation of the toggle.
func (t Toggle) String() string {
	switch t {
	case ToggleOn:
		return "on"
	case ToggleOff:
		return "off"
	default:
		return "auto"
	}
}

// IsAuto returns true if the toggle is set to auto-detect.
func (t Toggle) IsAuto() bool {
	return t == ToggleAuto || strings.EqualFold(string(t), "auto")
}

// IsOn returns true if the toggle is explicitly enabled.
func (t Toggle) IsOn() bool {
	return t == ToggleOn
}

// IsOff returns true if the toggle is explicitly disabled.
func (t Toggle) IsOff() bool {
	return t == ToggleOff
}

// ParseToggle parses a string value into a Toggle.
// Accepts: "auto", "", "on", "true", "enabled", "always", "off", "false", "disabled", "never"
func ParseToggle(s string) (Toggle, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return ToggleAuto, nil
	case "on", "true", "enabled", "always", "yes", "1":
		return ToggleOn, nil
	case "off", "false", "disabled", "never", "no", "0":
		return ToggleOff, nil
	default:
		return ToggleAuto, fmt.Errorf("invalid toggle value %q: must be auto, on, or off", s)
	}
}

// UnmarshalYAML implements yaml.Unmarshaler to support flexible input.
func (t *Toggle) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try to unmarshal as a boolean first
	var boolVal bool
	if err := unmarshal(&boolVal); err == nil {
		if boolVal {
			*t = ToggleOn
		} else {
			*t = ToggleOff
		}
		return nil
	}

	// Try to unmarshal as a string
	var strVal string
	if err := unmarshal(&strVal); err != nil {
		return fmt.Errorf("toggle must be a boolean or string: %w", err)
	}

	parsed, err := ParseToggle(strVal)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (t Toggle) MarshalYAML() (interface{}, error) {
	return t.String(), nil
}

// UnmarshalJSON implements json.Unmarshaler to support flexible input.
func (t *Toggle) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as a boolean first
	var boolVal bool
	if err := json.Unmarshal(data, &boolVal); err == nil {
		if boolVal {
			*t = ToggleOn
		} else {
			*t = ToggleOff
		}
		return nil
	}

	// Try to unmarshal as a string
	var strVal string
	if err := json.Unmarshal(data, &strVal); err != nil {
		return fmt.Errorf("toggle must be a boolean or string: %w", err)
	}

	parsed, err := ParseToggle(strVal)
	if err != nil {
		return err
	}
	*t = parsed
	return nil
}

// MarshalJSON implements json.Marshaler.
func (t Toggle) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}
