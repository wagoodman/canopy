package gotest

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// events that are not covered by JSONL since these are reported to STDERR and not STDOUT:
// - syntax error: e.g. "internal/example/hobbies.go:10:1: syntax error: non-declaration statement outside function body"
// - no go files due to wrong package test specifier: e.g.  "go test internal" -> "package internal: no Go files in /Users/wagoodman/.asdf/installs/golang/1.20.4/go/src/internal"

// JSONL represents a single line of JSON output from `go test -json`. It provides
// the raw structured data that gets parsed into Events for further processing.
type JSONL struct {
	Index   int64   `json:"-"`
	Raw     string  `json:"-"`
	Time    string  `json:"Time"`
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test,omitempty"`
	Elapsed float64 `json:"Elapsed,omitempty"`
	Output  string  `json:"Output,omitempty"`
	Error   error
}

// String formats the JSONL for human-readable display, showing the key information
// in a compact format. Used primarily for debugging and logging.
func (t JSONL) String() string {
	if t.Error != nil {
		return fmt.Sprintf("error: %v", t.Error)
	}
	return fmt.Sprintf("%s(%s): %s %q", t.Package, t.Test, t.Action, t.Output)
}

// NewJSONL parses a raw line from `go test -json` output into a structured JSONL object.
// Handles special edge cases where Go outputs non-JSON lines for build/setup failures
// that still need to be captured as events. Returns JSONL with Error field set if parsing fails.
func NewJSONL(ogLine string, idx int64) JSONL {
	line := strings.TrimSpace(ogLine)
	if strings.HasPrefix(line, "FAIL") {
		switch {
		case strings.HasSuffix(line, "[setup failed]"):
			// special case:
			//    | # github.com/wagoodman/canopy/cmd/canopy/cli/ui/simpletree/testprogress
			//    | package github.com/wagoodman/canopy/cmd/canopy/cli/ui/simpletree/testprogress (test)
			//    |        cmd/canopy/cli/ui/simpletree/testprogress/model_test.go:14:2: use of internal package github.com/anchore/bubbly/bubbles/internal/testutil not allowed
			//    | FAIL    github.com/wagoodman/canopy/cmd/canopy/cli/ui/simpletree/testprogress [setup failed]
			//
			// https://github.com/golang/go/blob/c71cbd544e3da139badd4c03612af41b63711705/src/cmd/go/internal/test/test.go#L915

			pkgName := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(line, "[setup failed]", ""), "FAIL", ""))
			return JSONL{
				Index:   idx,
				Raw:     ogLine,
				Time:    time.Now().Format(time.RFC3339Nano), // no good answer for this
				Action:  string(FailAction),
				Package: pkgName,
				Test:    "",
				Output:  line,
			}

		case strings.HasSuffix(line, "[build failed]"):
			// special case:
			// e.g. syntax error in package that is imported results in something like "FAIL    github.com/wagoodman/canopy/internal/example/math [build failed]"
			// https://github.com/golang/go/blob/c71cbd544e3da139badd4c03612af41b63711705/src/cmd/go/internal/test/test.go#L1224
			pkgName := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(line, "[build failed]", ""), "FAIL", ""))
			return JSONL{
				Index:   idx,
				Raw:     ogLine,
				Time:    time.Now().Format(time.RFC3339Nano), // no good answer for this
				Action:  string(FailAction),
				Package: pkgName,
				Test:    "",
				Output:  line,
			}
		}
	}
	var event JSONL
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return JSONL{Index: idx, Raw: ogLine, Error: fmt.Errorf("unable to unmarshal go test JSONL: %v", err)}
	}
	event.Index = idx
	event.Raw = ogLine
	return event
}
