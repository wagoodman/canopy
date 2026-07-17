package gotest

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strings"
	"sync"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/debug"
	"github.com/wagoodman/canopy/cmd/canopy/internal/cover"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
)

type ErrRunStderr struct {
	Output string
}

func (e ErrRunStderr) Error() string {
	return fmt.Sprintf("stderr from go test: %s", e.Output)
}

// RunnerConfig specifies how tests should be executed.
type RunnerConfig struct {
	Packages    *golist.PackageCollection
	Coverage    bool
	CoverageDir string // absolute path to persistent binary coverage directory (set externally when Coverage is true)
	NoCache     bool
	UserArgs    []string
	OnlyRefs    []Reference
	// Fingerprint captures the execution conditions (seed, race, tags, toolchain, env) so a repro
	// can recreate them. persisted as part of this config's JSON blob; nil on runs recorded before
	// fingerprinting existed.
	Fingerprint *ExecFingerprint `json:"Fingerprint,omitempty"`
}

// Runner coordinates test execution by spawning `go test` subprocesses and processing
// their JSON output into structured events and results.
type Runner struct {
	config RunnerConfig
}

// NewRunner creates a test runner with the specified configuration.
func NewRunner(config RunnerConfig) *Runner {
	return &Runner{
		config: config,
	}
}

// Run executes tests synchronously and returns the complete results.
// Blocks until all tests complete or an error occurs. Use Start() for async execution.
func (r *Runner) Run(ctx context.Context, resultConfig ResultConfig, onEvent ...func(*Event)) (*Run, error) {
	run, errs := r.Start(ctx, resultConfig, onEvent...)

	for err := range errs {
		if err != nil {
			return nil, err
		}
	}

	return run, nil
}

// Start executes tests asynchronously and returns immediately with a Run and error channel.
// Events are processed in real-time and sent to provided callbacks. The error channel
// will receive nil when execution completes successfully, or an error if something fails.
func (r *Runner) Start(ctx context.Context, resultConfig ResultConfig, onEvent ...func(*Event)) (*Run, <-chan error) {
	run := NewRun(r.config)
	run.Result = *NewResult(resultConfig)
	done := make(chan error)

	events, err := r.startEventStream(ctx)
	if err != nil {
		go func() {
			done <- fmt.Errorf("error running go test: %v", err)
		}()
		return nil, done
	}

	go func() {
		defer func() {
			for _, fn := range onEvent {
				fn(nil)
			}
			close(done)
		}()

	loop:
		for {
			select {
			case <-ctx.Done():
				// mark the run as canceled so the summary reflects the interruption rather than a false PASS.
				// this is published to the UI via the run-end event (onEvent(nil) in the deferred cleanup).
				run.Canceled = true
				// TODO: better error here?
				done <- fmt.Errorf("context cancelled")
				return
			case jsonl, ok := <-events:
				if !ok {
					break loop
				}
				e := NewEvent(run.ID, jsonl, r.config.Packages)
				if e == nil {
					log.Warn("empty test event discovered")
					continue
				}

				event := *e

				for _, fn := range onEvent {
					fn(&event)
				}

				run.Result.Update(event)
			}
		}

		if err := r.recordCoverage(run); err != nil {
			done <- err
			return
		}
	}()

	return run, done
}

// recordCoverage calculates and attaches package/function coverage to run when a coverage dir
// is configured. it is a no-op when coverage is disabled.
func (r *Runner) recordCoverage(run *Run) error {
	if r.config.CoverageDir == "" {
		return nil
	}

	pkgs, err := cover.PackageCoverage(r.config.CoverageDir)
	if err != nil {
		return fmt.Errorf("error calculating package coverage: %v", err)
	}

	funcs, overallPercent, err := cover.FunctionCoverage(r.config.CoverageDir)
	if err != nil {
		return fmt.Errorf("error calculating function coverage: %v", err)
	}

	// only record coverage when covdata actually produced data. an empty coverage dir
	// (e.g. a build failure) yields empty results with a nil error; setting coverage
	// anyway would fabricate a bogus 0.0%. leave it unset so Coverage() returns (_, false).
	if len(pkgs) > 0 || len(funcs) > 0 {
		run.Result.coverage = &overallPercent
	}
	run.PackageCoverage = pkgs
	run.FunctionCoverage = funcs

	return nil
}

func (r *Runner) startEventStream(ctx context.Context) (<-chan JSONL, error) { //nolint:funlen
	events := make(chan JSONL)

	initArgs := []string{"test"}
	if r.config.Coverage {
		initArgs = append(initArgs, "-cover")
	}
	initArgs = append(initArgs, "-json")

	if r.config.NoCache {
		initArgs = append(initArgs, "-count=1")
	}

	debug.SetLine(fmt.Sprintf("refs: %v", r.config.OnlyRefs))

	var args []string
	args = append(args, initArgs...)
	args = append(args, r.config.UserArgs...)
	args = append(args, runFilters(r.config.OnlyRefs)...)

	// -args must come last: it separates `go test` flags from test binary flags
	if r.config.CoverageDir != "" {
		args = append(args, "-args", fmt.Sprintf("-test.gocoverdir=%s", r.config.CoverageDir))
	}

	// use CommandContext so a cancelled ctx kills the child `go test` (and its process group)
	// instead of orphaning it.
	cmd := exec.CommandContext(ctx, "go", args...)

	log.WithFields("cmd", fmt.Sprintf("%q", strings.Join(cmd.Args, " "))).Trace("executing")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting command: %v", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	// send delivers to events but bails on ctx cancel so reader goroutines don't block forever on
	// an undrained channel once the consuming loop has returned.
	send := func(j JSONL) {
		select {
		case events <- j:
		case <-ctx.Done():
		}
	}

	go func() {
		defer wg.Done()
		jsonLFromReader(ctx, stdout, events)
	}()

	var sb strings.Builder
	go func() {
		defer wg.Done()

		reader := bufio.NewReader(stderr)
		for {
			line, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				log.WithFields("error", err).Warn("error reading test stderr")
				return
			}

			if line == "" {
				break
			}

			sb.WriteString(line + "\n")
		}
	}()

	go func() {
		// wait until BOTH pipes are fully drained before reaping: cmd.Wait closes the pipe
		// fds, so calling it while the stderr goroutine is still reading (the old ordering)
		// truncated stderr and surfaced a spurious "file already closed" error.
		wg.Wait()

		if err := cmd.Wait(); err != nil {
			// handle exit gracefully (0 or non-0)
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				// note: exit code 1 means there was a test failure
				rc := exitErr.ExitCode()
				if rc != 0 && rc != 1 {
					send(JSONL{Index: math.MaxInt64, Error: fmt.Errorf("error running command: %v", exitErr)})
				}
			} else {
				send(JSONL{Index: math.MaxInt64, Error: fmt.Errorf("error running command: %v", err)})
			}
		}

		if sb.Len() > 0 {
			send(JSONL{Index: math.MaxInt64, Error: ErrRunStderr{Output: sb.String()}})
		}

		close(events)
	}()

	return events, nil
}

func runFilters(refs []Reference) []string {
	if len(refs) == 0 {
		return nil
	}

	// go test honors only the LAST -run flag, so multiple per-ref flags would silently drop all
	// but one ref. Join the anchored per-ref patterns into a single alternation instead.
	// ponytail: for the common function-level case (^A$|^B$) this is exact. mixing subtest depths
	// in one alternation is imperfect since `/` is go test's level separator, but each ref pattern
	// is fully anchored so a function-level ref won't accidentally match a subtest of another ref.
	var patterns []string
	for _, ref := range refs {
		str := refString(ref)
		if str == "" {
			continue
		}
		patterns = append(patterns, str)
	}

	if len(patterns) == 0 {
		return nil
	}

	args := []string{fmt.Sprintf("-run=%s", strings.Join(patterns, "|"))}

	debug.SetLine(fmt.Sprintf("running tests: %v", args))

	return args
}

func refString(ref Reference) string {
	if ref.Package == "" || ref.IsPackage() {
		return ""
	}

	if ref.TRunName == "" {
		return fmt.Sprintf("^%s$", ref.FuncName)
	}

	return fmt.Sprintf("^%s/%s$", ref.FuncName, ref.TRunName)
}

func jsonLFromReader(ctx context.Context, stdout io.Reader, events chan<- JSONL) {
	reader := bufio.NewReader(stdout)
	var idx int64 = 1
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			select {
			case events <- JSONL{Index: math.MaxInt64, Error: fmt.Errorf("error reading input: %v", err)}:
			case <-ctx.Done():
			}
			return
		}

		if line == "" {
			break
		}

		select {
		case events <- NewJSONL(line, idx):
		case <-ctx.Done():
			return
		}
		idx++
	}
}
