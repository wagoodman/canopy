package ui

import (
	"io"
	"os"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/json"

	"github.com/anchore/clio"
)

func NewTestJSONUI(cfg TestUIConfig) clio.UI {
	var reportWriter io.WriteCloser
	if cfg.Writer != nil {
		reportWriter = cfg.Writer
	} else {
		reportWriter = os.Stdout
	}

	ux := newCoreUI().
		withNotifications().
		withReports().
		withHandlers(json.NewHandler(reportWriter)).
		withStdout(reportWriter)

	return ux
}
