package shard

// ReportVersion versions the join report JSON, which is a stable contract for scripts.
const ReportVersion = 1

// problem kinds, a closed set. Each maps to exactly one exit code (see kindExit).
const (
	KindUnreadableReceipt  = "unreadable_receipt"
	KindMissingShard       = "missing_shard"
	KindDuplicateShard     = "duplicate_shard"
	KindTotalMismatch      = "total_mismatch"
	KindVersionMismatch    = "version_mismatch"
	KindInputMismatch      = "input_mismatch"
	KindNeverRan           = "never_ran"
	KindRanTwice           = "ran_twice"
	KindPlannedNotReported = "planned_not_reported"
	KindCoverageBelow      = "coverage_below"
	KindCoverageMissing    = "coverage_missing"
	KindGateConflict       = "gate_conflict"
	KindShardFailed        = "shard_failed"
)

const (
	ExitPass        = 0
	ExitTestsFailed = 1
	ExitCannotRun   = 2
	ExitUnverified  = 3
	ExitGateFailed  = 4
)

const (
	CheckTests    = "tests"
	CheckVerified = "verified"
	CheckCoverage = "coverage"
)

// where the enforced covermin came from
const (
	ThresholdJoin     = "join"
	ThresholdReceipts = "receipts"
)

var kindExit = map[string]int{
	KindUnreadableReceipt:  ExitCannotRun,
	KindMissingShard:       ExitUnverified,
	KindDuplicateShard:     ExitUnverified,
	KindTotalMismatch:      ExitUnverified,
	KindVersionMismatch:    ExitUnverified,
	KindInputMismatch:      ExitUnverified,
	KindNeverRan:           ExitUnverified,
	KindRanTwice:           ExitUnverified,
	KindPlannedNotReported: ExitUnverified,
	KindCoverageBelow:      ExitGateFailed,
	KindCoverageMissing:    ExitGateFailed,
	KindGateConflict:       ExitGateFailed,
	KindShardFailed:        ExitTestsFailed,
}

// exitRank orders exit codes by how fundamental they are: 2, then 3, then 4, then 1.
var exitRank = map[int]int{ExitCannotRun: 4, ExitUnverified: 3, ExitGateFailed: 2, ExitTestsFailed: 1}

// Report is the result of a join. Every output (text, github-summary, json) renders this one value,
// and its JSON form is the versioned join report.
type Report struct {
	Version  int    `json:"version"`
	Result   string `json:"result"` // "pass" or "fail"
	ExitCode int    `json:"exit_code"`
	Total    int    `json:"total"`
	Packages int    `json:"packages"` // size of the unit list
	// Digests groups the present shards by overall input digest, the largest group first. One entry
	// means every shard ran the same plan.
	Digests    []DigestGroup `json:"digests"`
	Checks     Checks        `json:"checks"`
	Shards     []ShardReport `json:"shards"` // always Total long, Shards[i] is shard i+1
	Problems   []Problem     `json:"problems"`
	Warnings   []string      `json:"warnings"` // never affect the exit code (e.g. canopy version skew)
	Suggestion *Suggestion   `json:"suggestion,omitempty"`
}

type DigestGroup struct {
	Digest string `json:"digest"`
	Shards []int  `json:"shards"`
}

type Checks struct {
	Tests    TestsCheck    `json:"tests"`
	Verified VerifiedCheck `json:"verified"`
	Coverage CoverageCheck `json:"coverage"`
	Metrics  MetricsCheck  `json:"metrics"`
}

// TestsCheck sums the test tallies of every present shard.
type TestsCheck struct {
	OK bool `json:"ok"`
	TestTally
}

type VerifiedCheck struct {
	OK       bool `json:"ok"`
	Receipts int  `json:"receipts"` // shards with a receipt
	RanOnce  int  `json:"ran_once"` // units that ran in exactly one shard
}

type CoverageCheck struct {
	OK      bool     `json:"ok"`
	Enabled bool     `json:"enabled"`           // at least one shard collected coverage
	Percent *float64 `json:"percent,omitempty"` // over the merged profile
	// Threshold is the covermin that was enforced, from ThresholdSource ("join" or "receipts").
	Threshold       *float64 `json:"threshold,omitempty"`
	ThresholdSource string   `json:"threshold_source,omitempty"`
	// ReceiptThreshold is what the shards recorded, kept when the join's own value overrode it.
	ReceiptThreshold *float64 `json:"receipt_threshold,omitempty"`
	Profile          string   `json:"profile,omitempty"` // merged coverage.out
}

// MetricsCheck never fails the join; Warning says why nothing was written.
type MetricsCheck struct {
	OK       bool   `json:"ok"`
	Written  bool   `json:"written"`
	Path     string `json:"path,omitempty"`
	Packages int    `json:"packages"` // packages with a fresh sample from this join
	Warning  string `json:"warning,omitempty"`
}

type ShardReport struct {
	Index   int  `json:"index"`
	Present bool `json:"present"`
	*ShardResult
}

// ShardResult is what a present shard's receipt said.
type ShardResult struct {
	CanopyVersion string       `json:"canopy_version"`
	Digest        string       `json:"digest"`
	Weights       ShardWeights `json:"weights"`
	// EstimatedMS is the shard's planned load in ms, set only when weights came from metrics.
	EstimatedMS    *int64    `json:"estimated_ms,omitempty"`
	Planned        []string  `json:"planned"`
	Reported       []string  `json:"reported"`
	Passed         bool      `json:"passed"`
	ElapsedMS      int64     `json:"elapsed_ms"`
	Tests          TestTally `json:"tests"`
	Failures       []Failure `json:"failures"`
	FailedPackages []string  `json:"failed_packages,omitempty"`
	Coverprofile   string    `json:"coverprofile,omitempty"`
}

type ShardWeights struct {
	Source      string `json:"source"` // SourceMetrics or SourceStatic
	Measured    int    `json:"measured"`
	Estimated   int    `json:"estimated"`
	MetricsFile string `json:"metrics_file,omitempty"` // digest of the metrics file the shard read
	Ignored     string `json:"ignored,omitempty"`      // why metrics weren't used
}

// Problem is one finding. Kind is from the closed set above and decides the exit code; Shards and
// Packages say what it's about, and the embedded LineDiff (baseline, differing, only_in, values) is
// set for input_mismatch only.
type Problem struct {
	Check    string   `json:"check"`
	Kind     string   `json:"kind"`
	Shards   []int    `json:"shards,omitempty"`
	Packages []string `json:"packages,omitempty"`
	Group    string   `json:"group,omitempty"` // input group for input_mismatch
	*LineDiff
	Hint    string `json:"hint,omitempty"`
	Message string `json:"message"`
}

// Suggestion is the shard count advice. With no timing data Static is set, Note says so, and only
// the per-shard package counts in Estimates are meaningful.
type Suggestion struct {
	Current   int                  `json:"current"`
	Best      int                  `json:"best"`
	Static    bool                 `json:"static"`
	Note      string               `json:"note,omitempty"`
	Slowest   string               `json:"slowest_package,omitempty"` // sets the floor on wall time
	SlowestMS int64                `json:"slowest_ms,omitempty"`
	Estimates []SuggestionEstimate `json:"estimates"`
}

type SuggestionEstimate struct {
	Shards   int   `json:"shards"`
	WallMS   int64 `json:"wall_ms,omitempty"`
	RunnerMS int64 `json:"runner_ms,omitempty"`
	Packages []int `json:"packages"` // package count per shard
}

// finish sets the check verdicts, exit code and result from the problems found.
func (r *Report) finish() {
	r.Checks.Tests.OK, r.Checks.Verified.OK, r.Checks.Coverage.OK = true, true, true
	for _, p := range r.Problems {
		switch p.Check {
		case CheckTests:
			r.Checks.Tests.OK = false
		case CheckVerified:
			r.Checks.Verified.OK = false
		case CheckCoverage:
			r.Checks.Coverage.OK = false
		}
		if code := kindExit[p.Kind]; exitRank[code] > exitRank[r.ExitCode] {
			r.ExitCode = code
		}
	}
	r.Result = "pass"
	if r.ExitCode != ExitPass {
		r.Result = "fail"
	}
}
