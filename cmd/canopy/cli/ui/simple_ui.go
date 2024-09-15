package ui

import (
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/adapter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/clio"
)

var _ clio.UI = (*simpleUI)(nil)

type simpleUI struct {
	presenters     []presenter.Presenter
	handlers       []partybus.Handler
	teardownCalled bool
}

func newSimpleUI() *simpleUI {
	return &simpleUI{}
}

func (n *simpleUI) withNotifications() *simpleUI {
	return n.withHandledPresenters(adapter.NewAllEvents(event.CLINotification, presenter.NewNotificationEvent))
}

func (n *simpleUI) withReports() *simpleUI {
	return n.withHandledPresenters(adapter.NewAllEvents(event.CLIReport, presenter.NewReportEvent))
}

func (n *simpleUI) withHandledPresenters(adapters ...adapter.HandledPresenter) *simpleUI {
	for _, a := range adapters {
		n.handlers = append(n.handlers, a)
		n.presenters = append(n.presenters, a)
	}
	return n
}

func (n *simpleUI) withHandlers(handlers ...partybus.Handler) *simpleUI {
	n.handlers = append(n.handlers, handlers...)
	return n
}

// func (n *simpleUI) withPresenters(presenters ...presenter.Presenter) *simpleUI {
//	n.presenters = append(n.presenters, presenters...)
//	return n
//}

func (n *simpleUI) Setup(_ partybus.Unsubscribable) error {
	return nil
}

func (n *simpleUI) Handle(e partybus.Event) error {
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

func (n *simpleUI) Teardown(_ bool) error {
	var errs error
	if n.teardownCalled {
		return nil
	}

	n.teardownCalled = true

	for _, p := range n.presenters {
		if err := p.Present(os.Stdout, os.Stderr); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}
