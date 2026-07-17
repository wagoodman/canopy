package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"

	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/canopy/cmd/canopy/cli"
	"github.com/wagoodman/canopy/cmd/canopy/cli/commands"
	"github.com/wagoodman/canopy/cmd/canopy/internal"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"

	"github.com/anchore/clio"
	"github.com/anchore/go-sync"
)

const valueNotProvided = "[not provided]"

// exitCodeInterrupted is the conventional exit status for a process terminated by SIGINT (128 + signal 2).
const exitCodeInterrupted = 130

// all variables here are provided as build-time arguments, with clear default values
var version = "dev"
var gitCommit = valueNotProvided
var gitDescription = valueNotProvided
var buildDate = valueNotProvided

func main() {
	cmd := cli.New(
		clio.Identification{
			Name:           internal.ApplicationName,
			Version:        version,
			GitCommit:      gitCommit,
			GitDescription: gitDescription,
			BuildDate:      buildDate,
		},
	)

	// drive application control from a single context which can be cancelled (notifying the event loop to stop)
	ctx, cancel := context.WithCancel(context.Background())

	// set up executors for each dimension we want to coordinate CPU bounds for
	if !sync.HasContextExecutor(ctx, internal.ExecutorTestRunner) {
		ctx = sync.SetContextExecutor(ctx, internal.ExecutorTestRunner, sync.NewExecutor(runtime.NumCPU()*4))
	}

	cmd.SetContext(ctx)

	// note: it is important to always do signal handling from the main package. In this way if quill is used
	// as a lib a refactor would not need to be done (since anything from the main package cannot be imported this
	// nicely enforces this constraint)
	signals := make(chan os.Signal, 10) // Note: A buffered channel is recommended for this; see https://golang.org/pkg/os/signal/#Notify
	signal.Notify(signals, os.Interrupt)

	var exitCode int

	defer func() {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()

	defer func() {
		signal.Stop(signals)
		cancel()
	}()

	go func() {
		select {
		case <-signals: // first signal, cancel context
			log.Trace("signal interrupt, stop requested")
			bus.MarkInterrupted()
			cancel()
		case <-ctx.Done():
		}
		<-signals // second signal, hard exit
		log.Trace("signal interrupt, killing")
		os.Exit(1)
	}()

	if err := cmd.Execute(); err != nil {
		showError := func() {
			msg := err.Error()
			if msg != "" {
				style := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
				fmt.Fprintf(os.Stderr, "%s\n", style.Render("error: "+err.Error()))
			}
		}
		var sErr commands.SilentError
		if errors.As(err, &sErr) {
			if !sErr.IsSilent() {
				showError()
			}
		} else {
			showError()
		}

		exitCode = 1
	}

	// a user interrupt (ctrl-c) is a graceful exit as far as clio is concerned, so the worker error is dropped
	// and Execute returns nil. surface it as the conventional "terminated by SIGINT" code (128 + 2) so callers
	// (CI, scripts) don't mistake a canceled run for a passing one.
	if exitCode == 0 && bus.Interrupted() {
		exitCode = exitCodeInterrupted
	}
}
