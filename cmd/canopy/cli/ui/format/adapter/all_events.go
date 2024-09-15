package adapter

import (
	"io"

	"github.com/hashicorp/go-multierror"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/go-partybus"
)

type AllEvents struct {
	*handler.Aggregator
	factory presenter.EventFactory
}

func NewAllEvents(ty partybus.EventType, p presenter.EventFactory) *AllEvents {
	return &AllEvents{
		Aggregator: handler.NewAggregator(ty),
		factory:    p,
	}
}

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
