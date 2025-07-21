package references

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilter(t *testing.T) {
	testStrings := []string{
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_Handle",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_OnGoTestEvent",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_String",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestNewMultiPackageHandler",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietHandler",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietPackage",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestVerboseHandler",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestVerbosePackage",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/internal",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/internal/TestIndentWriter_Write",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/internal/TestIndentWriter_Write_ErrorHandling",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/internal/TestIndentWriter_Write_MultipleWrites",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter/TestGoTestResultSummary_Present",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter/TestJestTestResultSummary_Present",
		"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter/TestSplitWhitespace",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestEvent_Copy",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestEvent_HasAnnotation",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestExtractAnnotations",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestFindDefinitions",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestJSONL_String",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestMinimizeSelection",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestNewEvent",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestNewJSONL",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestNewResult",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestReference_Parent",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestReference_String",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestReference_rewriteTestName",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_ReferenceConclusiveAction",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_ReferenceEvents",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_References",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_ReferencesByAction",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_SetCoverage",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_TestReferencesByAction",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_TestStats",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_Update_WithPackage",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_Update_WithTest",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_Update_WithTest_StartConditions",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/Test_dfsTreeIterator_Next",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/Test_matcher_unique",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output/TestHasFailedPackageMarking",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output/TestHasPackageCoverageMarking",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output/TestHasPassedPackageMarking",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output/TestHasTestPassMarking",
		"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output/TestIsLogLine",
		"github.com/wagoodman/canopy/cmd/canopy/internal/ide/TestGoland_FileAtLineURL",
		"github.com/wagoodman/canopy/cmd/canopy/internal/ide/TestGoland_OpenFileAtLineCommand",
		"github.com/wagoodman/canopy/cmd/canopy/internal/ide/TestGoland_isActive",
		"github.com/wagoodman/canopy/cmd/canopy/internal/ide/TestNewGoland",
		"github.com/wagoodman/canopy/cmd/canopy/internal/ide/TestNewSnapshotEnvironmentGetterFromOSEnv",
	}

	tests := []struct {
		name            string
		term            string
		targets         []string
		expectedMatches []string // expected matching strings
		wantErr         require.ErrorAssertionFunc
	}{
		{
			name:    "empty filter returns all items",
			term:    "",
			targets: testStrings[:5],
			expectedMatches: []string{
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_Handle",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_OnGoTestEvent",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_String",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestNewMultiPackageHandler",
			},
		},
		{
			name:    "partial path match - cmd/canopy/cli",
			term:    "cmd/canopy/cli",
			targets: testStrings,
			// should match items containing this path segment
			expectedMatches: []string{
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_Handle",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_OnGoTestEvent",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_String",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestNewMultiPackageHandler",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietHandler",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietPackage",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestVerboseHandler",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestVerbosePackage",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/internal",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/internal/TestIndentWriter_Write",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/internal/TestIndentWriter_Write_ErrorHandling",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/internal/TestIndentWriter_Write_MultipleWrites",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter/TestGoTestResultSummary_Present",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter/TestJestTestResultSummary_Present",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter/TestSplitWhitespace",
			},
		},
		{
			name:    "test name match - TestMultiPackageHandler",
			term:    "TestMultiPackageHandler",
			targets: testStrings,
			// should match test names containing this string
			expectedMatches: []string{
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_Handle",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_OnGoTestEvent",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_String",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestNewMultiPackageHandler",
			},
		},
		{
			name:    "partial test name - TestQuiet",
			term:    "TestQuiet",
			targets: testStrings,
			// should match TestQuietHandler and TestQuietPackage
			expectedMatches: []string{
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietHandler",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietPackage",
			},
		},
		{
			name:    "specific package path - internal/gotest",
			term:    "internal/gotest",
			targets: testStrings,
			// should match all items with internal/gotest in path
			expectedMatches: []string{
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestEvent_Copy",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestEvent_HasAnnotation",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestExtractAnnotations",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestFindDefinitions",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestJSONL_String",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestMinimizeSelection",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestNewEvent",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestNewJSONL",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestNewResult",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestReference_Parent",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestReference_String",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestReference_rewriteTestName",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_ReferenceConclusiveAction",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_ReferenceEvents",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_References",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_ReferencesByAction",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_SetCoverage",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_TestReferencesByAction",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_TestStats",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_Update_WithPackage",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_Update_WithTest",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/TestResult_Update_WithTest_StartConditions",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/Test_dfsTreeIterator_Next",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/Test_matcher_unique",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output/TestHasFailedPackageMarking",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output/TestHasPackageCoverageMarking",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output/TestHasPassedPackageMarking",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output/TestHasTestPassMarking",
				"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output/TestIsLogLine",
				"github.com/wagoodman/canopy/cmd/canopy/internal/ide/TestGoland_FileAtLineURL",
				"github.com/wagoodman/canopy/cmd/canopy/internal/ide/TestGoland_OpenFileAtLineCommand",
				"github.com/wagoodman/canopy/cmd/canopy/internal/ide/TestGoland_isActive",
				"github.com/wagoodman/canopy/cmd/canopy/internal/ide/TestNewGoland",
				"github.com/wagoodman/canopy/cmd/canopy/internal/ide/TestNewSnapshotEnvironmentGetterFromOSEnv",
			},
		},
		{
			name:    "goxx package match",
			term:    "goxx",
			targets: testStrings,
			// should match items with goxx in path
			expectedMatches: []string{
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietHandler",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietPackage",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestVerboseHandler",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestVerbosePackage",
			},
		},
		{
			name:    "no matches for non-existent term",
			term:    "nonexistent",
			targets: testStrings,
			// should return empty result
			expectedMatches: []string{},
		},
		{
			name:    "case insensitive match",
			term:    "testmulti",
			targets: testStrings,
			// should match TestMultiPackageHandler variants
			expectedMatches: []string{
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_Handle",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_OnGoTestEvent",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_String",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestNewMultiPackageHandler",
			},
		},
		{
			name:    "fuzzy match with path separators",
			term:    "ui/format/handler",
			targets: testStrings,
			// should match items with this path structure
			expectedMatches: []string{
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_Handle",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_OnGoTestEvent",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestMultiPackageHandler_String",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/TestNewMultiPackageHandler",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietHandler",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestQuietPackage",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestVerboseHandler",
				"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx/TestVerbosePackage",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			result := filter(tt.term, tt.targets)

			// extract actual matched strings from result
			actualMatches := make([]string, len(result))
			for i, rank := range result {
				actualMatches[i] = tt.targets[rank.Index]
			}

			// verify expected matches
			if len(tt.expectedMatches) == 0 {
				require.Empty(t, actualMatches, "expected no matches")
				return
			}

			// check that all expected matches are present
			for _, expectedMatch := range tt.expectedMatches {
				found := false
				for _, actualMatch := range actualMatches {
					if actualMatch == expectedMatch {
						found = true
						break
					}
				}
				require.True(t, found, "expected %q to be in results, but got %v", expectedMatch, actualMatches)
			}

			// verify that matched indexes are populated when there's a filter term
			if tt.term != "" {
				for _, rank := range result {
					require.NotNil(t, rank.MatchedIndexes, "matched indexes should be populated for non-empty filter")
				}
			} else {
				// for empty filter, matched indexes should be nil
				for _, rank := range result {
					require.Nil(t, rank.MatchedIndexes, "matched indexes should be nil for empty filter")
				}
			}
		})
	}
}
