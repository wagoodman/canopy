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

	events, err := r.startEventStream()
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

		if r.config.CoverageDir != "" {
			pkgs, err := cover.PackageCoverage(r.config.CoverageDir)
			if err != nil {
				done <- fmt.Errorf("error calculating package coverage: %v", err)
				return
			}

			funcs, overallPercent, err := cover.FunctionCoverage(r.config.CoverageDir)
			if err != nil {
				done <- fmt.Errorf("error calculating function coverage: %v", err)
				return
			}

			run.Result.coverage = &overallPercent
			run.PackageCoverage = pkgs
			run.FunctionCoverage = funcs
		}
	}()

	return run, done
}

func (r *Runner) startEventStream() (<-chan JSONL, error) { //nolint:funlen
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

	cmd := exec.Command("go", args...)

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

	go func() {
		defer wg.Done()

		jsonLFromReader(stdout, events)

		if err := cmd.Wait(); err != nil {
			// handle exit gracefully (0 or non-0)
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				// note: exit code 1 means there was a test failure
				rc := exitErr.ExitCode()
				if rc != 0 && rc != 1 {
					events <- JSONL{Index: math.MaxInt64, Error: fmt.Errorf("error running command: %v", exitErr)}
				}
			} else {
				events <- JSONL{Index: math.MaxInt64, Error: fmt.Errorf("error running command: %v", err)}
			}
		}
	}()

	var sb strings.Builder
	go func() {
		defer wg.Done()

		reader := bufio.NewReader(stderr)
		for {
			line, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				fmt.Println(err)
				return
			}

			if line == "" {
				break
			}

			sb.WriteString(line + "\n")
		}
	}()

	go func() {
		wg.Wait()

		if sb.Len() > 0 {
			events <- JSONL{Index: math.MaxInt64, Error: ErrRunStderr{Output: sb.String()}}
		}

		close(events)
	}()

	return events, nil
}

func runFilters(refs []Reference) []string {
	if len(refs) == 0 {
		return nil
	}

	var args []string
	for _, ref := range refs {
		str := refString(ref)
		if str == "" {
			continue
		}
		args = append(args, fmt.Sprintf("-run=%s", str))
	}

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

func jsonLFromReader(stdout io.Reader, events chan<- JSONL) {
	reader := bufio.NewReader(stdout)
	var idx int64 = 1
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			events <- JSONL{Index: math.MaxInt64, Error: fmt.Errorf("error reading input: %v", err)}
			return
		}

		if line == "" {
			break
		}

		events <- NewJSONL(line, idx)
		idx++
	}
}
