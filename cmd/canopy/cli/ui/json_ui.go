package ui

import (
	"github.com/anchore/clio"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/json"
	"io"
	"os"
)

func NewJSONUI(cfg Config) clio.UI {
	var reportWriter io.WriteCloser
	if cfg.Writer != nil {
		reportWriter = cfg.Writer
	} else {
		reportWriter = os.Stdout
	}

	ux := newSimpleUI().
		withNotifications().
		withReports().
		withHandlers(json.NewHandler(reportWriter)).
		withStdout(reportWriter)

	return ux
}
