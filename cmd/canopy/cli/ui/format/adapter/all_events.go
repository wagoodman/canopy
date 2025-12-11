package adapter

import (
	"io"

	"github.com/hashicorp/go-multierror"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/go-partybus"
)

// AllEvents aggregates all events of a specific type and presents them using
// the provided factory.
type AllEvents struct {
	// Aggregator collects events from the event bus.
	*handler.Aggregator

	// factory creates a presenter for each collected event.
	factory presenter.EventFactory
}

// NewAllEvents creates an adapter that collects all events of the specified type
// and presents them using the given presenter factory.
func NewAllEvents(ty partybus.EventType, p presenter.EventFactory) *AllEvents {
	return &AllEvents{
		Aggregator: handler.NewAggregator(ty),
		factory:    p,
	}
}

// Present writes all collected events to stdout/stderr using the presenter factory.
// Returns any accumulated presentation errors.
func (p AllEvents) Present(stdout, stderr io.Writer) error {
	var errs error
	for _, e := range p.Events() {
		pres := p.factory(e)
		if err := pres.Present(stdout, stderr); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}
