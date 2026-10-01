package shard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ReceiptVersion changes only when a field changes meaning; adding a field doesn't bump it.
const ReceiptVersion = 2

// Receipt is what one shard writes after it runs: what it was asked to run, with which inputs, and
// what actually happened. The join reads only receipts, so it needs no checkout.
type Receipt struct {
	Version       int                     `json:"version"`
	CanopyVersion string                  `json:"canopy_version"`
	Index         int                     `json:"index"`
	Total         int                     `json:"total"`
	Digest        string                  `json:"digest"`
	Inputs        map[string]ReceiptInput `json:"inputs"` // keyed by group name (plan, selection, run, ...)
	Metrics       ReceiptMetrics          `json:"metrics"`
	Runner        ReceiptRunner           `json:"runner"`
	Units         []Unit                  `json:"units"` // the planned units with the weights they were split by, sorted by package
	Planned       []string                `json:"planned"`
	LoadMS        int64                   `json:"load_ms"` // this shard's planned load (ms with metrics, test counts without)
	Reported      []string                `json:"reported"`
	Passed        bool                    `json:"passed"`
	ElapsedMS     int64                   `json:"elapsed_ms"`
	Tests         TestTally               `json:"tests"`
	Failures      []Failure               `json:"failures"`
	FailedPkgs    []string                `json:"failed_packages,omitempty"` // failed without a failing test (build or setup failure)
	Gates         Gates                   `json:"gates"`
	Observations  map[string]int64        `json:"observations"`
	Coverprofile  string                  `json:"coverprofile,omitempty"` // file name next to the receipt
}

// ReceiptInput is one digest group. Lines are left out for the selection package lines and for
// weights (too long); weights carry the source and counts instead.
type ReceiptInput struct {
	Digest    string   `json:"digest"`
	Lines     []string `json:"lines,omitempty"`
	Source    string   `json:"source,omitempty"`
	Measured  int      `json:"measured,omitempty"`
	Estimated int      `json:"estimated,omitempty"`
}

// ReceiptMetrics explains where the weights came from. It is not part of the digest.
type ReceiptMetrics struct {
	File    string   `json:"file,omitempty"` // sha256 of the metrics file bytes
	Env     Env      `json:"env"`
	Profile []string `json:"profile"`
	Ignored string   `json:"ignored"`
}

type ReceiptRunner struct {
	CPUs int `json:"cpus"`
}

type TestTally struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

type Failure struct {
	Package string `json:"package"`
	Test    string `json:"test"`
}

// Gates are the result gates a shard resolved and deferred to the join. Unset gates are nil.
type Gates struct {
	CoverMin *float64 `json:"covermin,omitempty"`
}

// Lines is the [gates] input group for these gates.
func (g Gates) Lines() []string {
	if g.CoverMin != nil {
		return []string{fmt.Sprintf("covermin %g", *g.CoverMin)}
	}
	return nil
}

// ReceiptPath is where shard i writes its receipt under the shard dir.
func ReceiptPath(shardDir string, index int) string {
	return filepath.Join(OutDir(shardDir), fmt.Sprintf("shard-%d.json", index))
}

// OutDir holds receipts and per-shard coverprofiles; it is what the workflow uploads.
func OutDir(shardDir string) string {
	return filepath.Join(shardDir, "out")
}

// WriteReceipt writes r atomically.
func WriteReceipt(path string, r Receipt) error {
	return writeJSON(path, r)
}

// LoadedReceipt is a receipt file read from disk, or the reason it couldn't be.
type LoadedReceipt struct {
	Path    string
	Receipt *Receipt
	Err     error
}

// LoadDir reads every shard-*.json in dir, sorted by path. A file that can't be parsed or has an
// unknown version is returned with Err set, so the join can report it instead of stopping.
func LoadDir(dir string) ([]LoadedReceipt, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "shard-*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []LoadedReceipt
	for _, p := range paths {
		lr := LoadedReceipt{Path: p}
		b, err := os.ReadFile(p)
		if err == nil {
			var r Receipt
			if err = json.Unmarshal(b, &r); err == nil && r.Version != ReceiptVersion {
				err = fmt.Errorf("unsupported receipt version %d", r.Version)
			}
			if err == nil {
				lr.Receipt = &r
			}
		}
		lr.Err = err
		out = append(out, lr)
	}
	return out, nil
}
