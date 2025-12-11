package golist

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
)

// processorFn is a callback function that processes the stdout of a `go list` command.
type processorFn func(output io.ReadCloser) error

// run executes `go list` with the given arguments and packages, streaming output to the processor function.
// It handles both successful and error exit codes from the go command.
func run(moreArgs []string, fn processorFn, pkgs ...string) error {
	args := []string{"list"}
	args = append(args, moreArgs...)
	args = append(args, pkgs...)

	cmd := exec.Command("go", args...)
	log.WithFields("cmd", fmt.Sprintf("%q", strings.Join(cmd.Args, " "))).Trace("executing")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %v", err)
	}

	if err := fn(stdout); err != nil {
		return fmt.Errorf("unable to process stdout: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		// handle exit gracefully (0 or non-0)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("error running command: %v", exitErr)
		}
		return fmt.Errorf("error running command: %v", err)
	}

	return nil
}
