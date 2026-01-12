package cienv

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupWriter_Write(t *testing.T) {
	var buf bytes.Buffer
	// Use GitHub CI type explicitly for predictable output
	gw := NewGroupWriterForCI(&buf, "Test Group", &Environment{Type: CITypeGitHub})

	n, err := gw.Write([]byte("line 1\n"))
	require.NoError(t, err)
	assert.Equal(t, 7, n)

	n, err = gw.Write([]byte("line 2\n"))
	require.NoError(t, err)
	assert.Equal(t, 7, n)

	// Buffer should be empty before flush
	assert.Empty(t, buf.String())
	assert.True(t, gw.HasContent())

	// Flush should write with group markers
	_, err = gw.Flush()
	require.NoError(t, err)

	expected := "::group::Test Group\nline 1\nline 2\n::endgroup::\n"
	assert.Equal(t, expected, buf.String())
	assert.False(t, gw.HasContent())
}

func TestGroupWriter_FlushEmpty(t *testing.T) {
	var buf bytes.Buffer
	gw := NewGroupWriter(&buf, "Empty Group")

	n, err := gw.Flush()
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Empty(t, buf.String())
}

func TestGroupWriter_Reset(t *testing.T) {
	var buf bytes.Buffer
	gw := NewGroupWriter(&buf, "Reset Test")

	_, _ = gw.Write([]byte("some content"))
	assert.True(t, gw.HasContent())

	gw.Reset()
	assert.False(t, gw.HasContent())

	_, _ = gw.Flush()
	assert.Empty(t, buf.String())
}

func TestGroupWriter_WriteString(t *testing.T) {
	var buf bytes.Buffer
	// Use GitHub CI type explicitly for predictable output
	gw := NewGroupWriterForCI(&buf, "String Test", &Environment{Type: CITypeGitHub})

	_, _ = gw.WriteString("hello world\n")
	_, _ = gw.Flush()

	expected := "::group::String Test\nhello world\n::endgroup::\n"
	assert.Equal(t, expected, buf.String())
}

func TestGroupWriter_AzurePipelines(t *testing.T) {
	var buf bytes.Buffer
	gw := NewGroupWriterForCI(&buf, "Azure Test", &Environment{Type: CITypeAzure})

	_, _ = gw.WriteString("azure content\n")
	_, _ = gw.Flush()

	expected := "##[group]Azure Test\nazure content\n##[endgroup]\n"
	assert.Equal(t, expected, buf.String())
}

func TestGroupWriter_GitLabCI(t *testing.T) {
	var buf bytes.Buffer
	gw := NewGroupWriterForCI(&buf, "GitLab Test", &Environment{Type: CITypeGitLab})

	_, _ = gw.WriteString("gitlab content\n")
	_, _ = gw.Flush()

	// GitLab output contains timestamps and section IDs, so just check key parts
	output := buf.String()
	assert.Contains(t, output, "section_start:")
	assert.Contains(t, output, "[collapsed=true]")
	assert.Contains(t, output, "GitLab Test")
	assert.Contains(t, output, "gitlab content")
	assert.Contains(t, output, "section_end:")
}

func TestGroupWriter_UnknownCI(t *testing.T) {
	var buf bytes.Buffer
	// Unknown CI type should just output content without markers
	gw := NewGroupWriterForCI(&buf, "No CI Test", &Environment{Type: CITypeUnknown})

	_, _ = gw.WriteString("plain content\n")
	_, _ = gw.Flush()

	// Content should be output without any CI markers
	expected := "plain content\n"
	assert.Equal(t, expected, buf.String())
}

func TestGroupWriter_NilEnvironment(t *testing.T) {
	var buf bytes.Buffer
	// Nil environment should just output content without markers
	gw := NewGroupWriterForCI(&buf, "Nil Env Test", nil)

	_, _ = gw.WriteString("plain content\n")
	_, _ = gw.Flush()

	expected := "plain content\n"
	assert.Equal(t, expected, buf.String())
}

func TestGroupConfig_IsEnabledWith(t *testing.T) {
	tests := []struct {
		name    string
		config  GroupConfig
		detect  func() *Environment
		want    bool
	}{
		{
			name:   "explicit on",
			config: GroupConfig{Enabled: ToggleOn},
			detect: func() *Environment { return nil },
			want:   true,
		},
		{
			name:   "explicit off in CI",
			config: GroupConfig{Enabled: ToggleOff},
			detect: func() *Environment { return &Environment{SupportsGrouping: true} },
			want:   false,
		},
		{
			name:   "auto-detect in CI with grouping",
			config: GroupConfig{Enabled: ToggleAuto},
			detect: func() *Environment { return &Environment{SupportsGrouping: true} },
			want:   true,
		},
		{
			name:   "auto-detect in CI without grouping",
			config: GroupConfig{Enabled: ToggleAuto},
			detect: func() *Environment { return &Environment{SupportsGrouping: false} },
			want:   false,
		},
		{
			name:   "auto-detect not in CI",
			config: GroupConfig{Enabled: ToggleAuto},
			detect: func() *Environment { return nil },
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.IsEnabledWith(tt.detect)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGroupConfig_ShouldGroup(t *testing.T) {
	tests := []struct {
		name   string
		config GroupConfig
		passed bool
		want   bool
	}{
		{
			name:   "passed with GroupPassedPackages true",
			config: GroupConfig{GroupPassedPackages: true},
			passed: true,
			want:   true,
		},
		{
			name:   "passed with GroupPassedPackages false",
			config: GroupConfig{GroupPassedPackages: false},
			passed: true,
			want:   false,
		},
		{
			name:   "failed with GroupFailedPackages true",
			config: GroupConfig{GroupFailedPackages: true},
			passed: false,
			want:   true,
		},
		{
			name:   "failed with GroupFailedPackages false",
			config: GroupConfig{GroupFailedPackages: false},
			passed: false,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ShouldGroup(tt.passed)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultGroupConfig(t *testing.T) {
	cfg := DefaultGroupConfig()

	assert.True(t, cfg.Enabled.IsAuto())
	assert.True(t, cfg.GroupPassedPackages)
	assert.False(t, cfg.GroupFailedPackages)
}
