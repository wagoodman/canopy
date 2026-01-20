package group

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoop(t *testing.T) {
	result := noop("Title", "content\n")
	assert.Equal(t, "content\n", result)
}

func TestGitHub(t *testing.T) {
	result := GitHub("Test Group", "line 1\nline 2\n")
	expected := "::group::Test Group\nline 1\nline 2\n::endgroup::\n"
	assert.Equal(t, expected, result)
}

func TestAzure(t *testing.T) {
	result := Azure("Azure Test", "azure content\n")
	expected := "##[group]Azure Test\nazure content\n##[endgroup]\n"
	assert.Equal(t, expected, result)
}

func TestGitLab(t *testing.T) {
	result := GitLab("GitLab Test", "gitlab content\n")

	// GitLab output contains timestamps and section IDs, so just check key parts
	assert.Contains(t, result, "section_start:")
	assert.Contains(t, result, "[collapsed=true]")
	assert.Contains(t, result, "GitLab Test")
	assert.Contains(t, result, "gitlab content")
	assert.Contains(t, result, "section_end:")
}

func TestGitLab_UniqueIDs(t *testing.T) {
	result1 := GitLab("First", "content1\n")
	result2 := GitLab("Second", "content2\n")

	// extract section names from results
	extractSectionName := func(s string) string {
		// format: section_start:TIMESTAMP:NAME[collapsed=true]
		start := strings.Index(s, "section_start:")
		if start == -1 {
			return ""
		}
		s = s[start+len("section_start:"):]
		// skip timestamp
		colonIdx := strings.Index(s, ":")
		if colonIdx == -1 {
			return ""
		}
		s = s[colonIdx+1:]
		// find end of name
		bracketIdx := strings.Index(s, "[")
		if bracketIdx == -1 {
			return ""
		}
		return s[:bracketIdx]
	}

	name1 := extractSectionName(result1)
	name2 := extractSectionName(result2)

	assert.NotEmpty(t, name1)
	assert.NotEmpty(t, name2)
	assert.NotEqual(t, name1, name2, "section names should be unique")
}
