package goxx

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

func TestQuietHandler(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
	}{
		{
			name:    "go1.21.3",
			fixture: "full/go1.21.3-default.jsonl",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			sb := strings.Builder{}
			cfg := QuietPackageConfig{
				Color:            false,
				PackageNameWidth: 150,
			}

			for _, b := range []bool{true, false} {
				t.Run(fmt.Sprintf("hide-no-tests=%v", b), func(t *testing.T) {
					cfg.HidePackagesWithNoTestFiles = b

					subject := NewQuietHandler(&sb, cfg)
					events := fixtureEvents(t, tt.fixture)
					for e := range events {
						err := subject.OnGoTestEvent(e)
						if errors.Is(err, handler.ErrPackageComplete) {
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

func TestQuietPackage(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		ref     gotest.Reference
	}{
		{
			name:    "failing package",
			fixture: "mixed-non-verbose.jsonl",
			ref: gotest.Reference{
				Package:  "github.com/wagoodman/canopy/internal/test-fixtures/weird.d",
				FuncName: "TestAddFailingSubtest",
				TRunName: "Test_weird_numbers_(oops)/offset=2",
			},
		},
		{
			name:    "passing package",
			fixture: "mixed-non-verbose.jsonl",
			ref: gotest.Reference{
				Package:  "github.com/wagoodman/canopy/cmd/canopy/internal/gotest",
				FuncName: "Test_dfsTreeIterator_Next",
				TRunName: "duplicate_case",
			},
		},
		{
			name:    "panic package",
			fixture: "panic-non-verbose.jsonl",
			ref: gotest.Reference{
				Package:  "github.com/wagoodman/canopy/internal/test-fixtures/panic",
				FuncName: "TestPanic",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			sb := strings.Builder{}
			cfg := QuietPackageConfig{
				Color:            false,
				PackageNameWidth: 150,
			}
			subject := NewQuietPackage(&sb, cfg, tt.ref)
			events := fixtureEvents(t, tt.fixture)
			for e := range events {
				err := subject.OnGoTestEvent(e)
				if errors.Is(err, handler.ErrPackageComplete) {
					// this one is OK to ignore
					continue
				}
				require.NoError(t, err)
			}

			output := sb.String() // usecase: to stdout
			snaps.MatchSnapshot(t, output)
			assert.Equal(t, output, subject.String()) // usecase: studio UI

		})
	}

}
