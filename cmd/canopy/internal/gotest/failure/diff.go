package failure

import (
	"regexp"
	"strconv"
	"strings"
)

// diffParser parses cmp.Diff and similar diff output.
type diffParser struct{}

var (
	// cmpDiffMarkerPattern matches diff direction markers like "(-want +got)" or "(-got +want)".
	cmpDiffMarkerPattern = regexp.MustCompile(`\(-\w+ \+\w+\)`)
	// diffLocationPattern matches file:line patterns in diff output.
	diffLocationPattern = regexp.MustCompile(`(\S+\.go):(\d+)`)
)

// diff line prefixes
const (
	additionPrefix = "+"
	removalPrefix  = "-"
)

func (p *diffParser) Name() string {
	return "diff"
}

func (p *diffParser) CanParse(output string) bool {
	// look for cmp.Diff markers like "(-want +got)" or "(-got +want)"
	return cmpDiffMarkerPattern.MatchString(output)
}

func (p *diffParser) Parse(output string) *StructuredFailure {
	sf := &StructuredFailure{
		FailureType: DiffFailure,
		RawOutput:   output,
		Diff: &DiffInfo{
			Version: DiffInfoVersion,
		},
	}

	lines := strings.Split(output, "\n")
	var inDiff bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// extract source location if present
		if sf.Location.IsZero() {
			if matches := diffLocationPattern.FindStringSubmatch(line); len(matches) >= 3 {
				sf.Location.File = matches[1]
				if lineNum, err := strconv.Atoi(matches[2]); err == nil {
					sf.Location.Line = lineNum
				}
			}
		}

		// detect start of diff section
		if cmpDiffMarkerPattern.MatchString(line) {
			inDiff = true
			continue
		}

		if !inDiff {
			continue
		}

		// skip empty lines and non-diff content
		if trimmed == "" || strings.HasPrefix(trimmed, "---") {
			continue
		}

		// categorize and append diff lines in order
		switch {
		case strings.HasPrefix(trimmed, additionPrefix) && !strings.HasPrefix(trimmed, "++"):
			content := strings.TrimPrefix(trimmed, additionPrefix)
			content = strings.TrimSpace(content)
			sf.Diff.Chunks = append(sf.Diff.Chunks, DiffChunks{
				Type:    DiffChunkAddition,
				Content: content,
			})
		case strings.HasPrefix(trimmed, removalPrefix) && !strings.HasPrefix(trimmed, "--"):
			content := strings.TrimPrefix(trimmed, removalPrefix)
			content = strings.TrimSpace(content)
			sf.Diff.Chunks = append(sf.Diff.Chunks, DiffChunks{
				Type:    DiffChunkRemoval,
				Content: content,
			})
		case !strings.HasPrefix(trimmed, "@"):
			// context line (unchanged)
			sf.Diff.Chunks = append(sf.Diff.Chunks, DiffChunks{
				Type:    DiffChunkContext,
				Content: trimmed,
			})
		}
	}

	return sf
}
