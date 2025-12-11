package golist

import (
	"bufio"
	"io"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
)

// type ErrRunStderr struct {
//	Output string
//}
//
// func (e ErrRunStderr) Error() string {
//	return fmt.Sprintf("stderr from go test: %s", e.Output)
//}

// PackageNames retrieves import paths for the given package patterns using `go list`.
// Returns a slice of package import paths (one per line of output).
// This is a faster alternative to PackageInfo when only import paths are needed.
func PackageNames(pkgs ...string) ([]string, error) {
	var output []string

	fn := func(stdout io.ReadCloser) error {
		reader := bufio.NewReader(stdout)
		for {
			line, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				log.WithFields("error", err).Warn("error reading from go list stdout")
				return err
			}

			if line == "" {
				break
			}

			output = append(output, strings.TrimSpace(line))
		}
		return nil
	}

	if err := run(nil, fn, pkgs...); err != nil {
		return nil, err
	}

	log.WithFields("count", len(output)).Trace("go list packages")

	return output, nil
}
