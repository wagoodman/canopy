package ui

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/adapter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/clio"
)

var _ clio.UI = (*coreUI)(nil)

type coreUI struct {
	presenters     []presenter.Presenter
	handlers       []partybus.Handler
	stdout         io.WriteCloser
	stderr         io.WriteCloser
	teardownCalled bool
}

func newCoreUI() *coreUI {
	// we always respond to simple print events by default (for debugging and such)
	return (&coreUI{}).withPrintEvents()
}

func (n *coreUI) withNotifications() *coreUI {
	return n.withHandledPresenters(adapter.NewAllEvents(event.CLINotification, presenter.NewNotificationEvent))
}

func (n *coreUI) withReports() *coreUI {
	return n.withHandledPresenters(adapter.NewAllEvents(event.CLIReport, presenter.NewReportEvent))
}

func (n *coreUI) withPrintEvents() *coreUI {
	return n.withHandledPresenters(adapter.NewAllEvents(event.PrintType, presenter.NewPrintEvent))
}

func (n *coreUI) withStdout(writer io.WriteCloser) *coreUI {
	n.stdout = writer
	return n
}

func (n *coreUI) withStderr(writer io.WriteCloser) *coreUI {
	n.stderr = writer
	return n
}

func (n *coreUI) withHandledPresenters(adapters ...adapter.HandledPresenter) *coreUI {
	for _, a := range adapters {
		n.handlers = append(n.handlers, a)
		n.presenters = append(n.presenters, a)
	}
	return n
}

func (n *coreUI) withHandlers(handlers ...partybus.Handler) *coreUI {
	n.handlers = append(n.handlers, handlers...)
	return n
}

// func (n *coreUI) withPresenters(presenters ...presenter.Presenter) *coreUI {
//	n.presenters = append(n.presenters, presenters...)
//	return n
//}

type nopCloser struct {
}

func (n *nopCloser) Close() error {
	return nil
}

func newNopWriteCloser(writer io.Writer) io.WriteCloser {
	return &struct {
		io.Writer
		io.Closer
	}{
		Writer: writer,
		Closer: &nopCloser{},
	}
}

func (n *coreUI) Setup(_ partybus.Unsubscribable) error {
	if n.stdout == nil {
		n.stdout = newNopWriteCloser(os.Stdout)
	}
	if n.stderr == nil {
		n.stderr = newNopWriteCloser(os.Stderr)
	}

	return nil
}

func (n *coreUI) Handle(e partybus.Event) error {
	// if n.teardownCalled {
	//	return nil
	//}
	var errs error
	for _, h := range n.handlers {
		if err := h.Handle(e); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

func (n *coreUI) Teardown(_ bool) error {
	var errs error
	if n.teardownCalled {
		return nil
	}

	n.teardownCalled = true

	// flush any buffered handler output before the presenters run. a streaming handler closes
	// its open collapsible group (writes ::endgroup::) here, so the summary the presenters emit
	// next lands outside the group instead of being collapsed with the passed/skipped packages.
	for _, h := range n.handlers {
		s, ok := h.(fmt.Stringer)
		if !ok {
			continue
		}
		if out := s.String(); out != "" && n.stdout != nil {
			if _, err := io.WriteString(n.stdout, out); err != nil {
				errs = multierror.Append(errs, err)
			}
		}
	}

	for _, p := range n.presenters {
		if err := p.Present(n.stdout, n.stderr); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	if n.stdout != nil {
		if err := n.stdout.Close(); err != nil {
			if !errors.Is(err, os.ErrClosed) {
				errs = multierror.Append(errs, fmt.Errorf("failed to close stdout: %w", err))
			}
		}
	}

	if n.stderr != nil {
		if err := n.stderr.Close(); err != nil {
			if !errors.Is(err, os.ErrClosed) {
				errs = multierror.Append(errs, fmt.Errorf("failed to close stderr: %w", err))
			}
		}
	}

	return errs
}
