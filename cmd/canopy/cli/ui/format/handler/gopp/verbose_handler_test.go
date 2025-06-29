package gopp

import (
	"errors"
	"fmt"
	error2 "github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestVerboseHandler(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
	}{
		{
			name:    "go1.21.3",
			fixture: "full/go1.21.3-verbose.jsonl",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			sb := strings.Builder{}
			cfg := VerbosePackageConfig{
				Color:            false,
				PackageNameWidth: 150,
			}

			for _, b := range []bool{true, false} {
				t.Run(fmt.Sprintf("hide-no-tests=%v", b), func(t *testing.T) {
					cfg.HidePackagesWithNoTestFiles = b

					subject := NewVerboseHandler(&sb, cfg)
					events := fixtureEvents(t, tt.fixture)
					for e := range events {
						err := subject.OnGoTestEvent(e)
						if errors.Is(err, error2.ErrPackageComplete) {
							// this one is OK to ignore
							continue
						}
						require.NoError(t, err)
					}

					snaps.MatchSnapshot(t, sb.String())
				})
			}
		})
	}
}

func TestVerbosePackage(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		ref     gotest.Reference
	}{
		{
			name:    "failing package",
			fixture: "mixed-verbose.jsonl",
			ref: gotest.Reference{
				Package:  "github.com/wagoodman/canopy/internal/test-fixtures/weird.d",
				FuncName: "TestAddFailingSubtest",
				TRunName: "Test_weird_numbers_(oops)/offset=2",
			},
		},
		{
			name:    "passing package",
			fixture: "mixed-verbose.jsonl",
			ref: gotest.Reference{
				Package:  "github.com/wagoodman/canopy/cmd/canopy/internal/gotest",
				FuncName: "Test_dfsTreeIterator_Next",
				TRunName: "duplicate_case",
			},
		},
		{
			name:    "panic package",
			fixture: "panic-verbose.jsonl",
			ref: gotest.Reference{
				Package:  "github.com/wagoodman/canopy/internal/test-fixtures/panic",
				FuncName: "TestPanic",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			sb := strings.Builder{}
			cfg := VerbosePackageConfig{
				Color:            false,
				PackageNameWidth: 150,
			}
			subject := NewVerbosePackage(&sb, cfg, tt.ref)
			events := fixtureEvents(t, tt.fixture)
			for e := range events {
				err := subject.OnGoTestEvent(e)
				if errors.Is(err, error2.ErrPackageComplete) {
					// this one is OK to ignore
					continue
				}
				require.NoError(t, err)
			}

			output := sb.String() // usecase: to stdout
			snaps.MatchSnapshot(t, output)
			assert.Empty(t, subject.String()) // usecase: to studio UI
		})
	}

}

func fixtureEvents(t testing.TB, name string) <-chan gotest.Event {
	fh, err := os.Open(filepath.Join("testdata", name))
	require.NoError(t, err)

	return gotest.ReplayEvents(fh, nil)
}

func TestHasTestPassMarking(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"--- PASS: TestExample", true},
		{"   --- PASS: TestExample", true},
		{"--- FAIL: TestExample", false},
		{"PASS: TestExample", false},
		{"", false},
		{"--- PASS:TestExample", true}, // Even without a space, it should detect
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			result := output.HasTestPassMarking(tt.output)
			assert.Equal(t, tt.expected, result, "Output: %q", tt.output)
		})
	}
}

func TestHasTestStartMarking(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"=== RUN   TestExample", true},
		{"=== RUN", true},
		{"--- RUN TestExample", false},
		{"RUN TestExample", false},
		{"", false},
		{"  === RUN TestExample", false}, // Leading spaces make it false
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			result := output.HasTestStartMarking(tt.output)
			assert.Equal(t, tt.expected, result, "Output: %q", tt.output)
		})
	}
}
