package handler

import (
	"github.com/wagoodman/go-partybus"
)

var _ partybus.Handler = (*Aggregator)(nil)

type Aggregator struct {
	ty     partybus.EventType
	events []partybus.Event
}

func NewAggregator(ty partybus.EventType) *Aggregator {
	return &Aggregator{
		ty: ty,
	}
}

func (a *Aggregator) Handle(e partybus.Event) error {
	if e.Type == a.ty {
		a.events = append(a.events, e)
	}
	return nil
}

func (a Aggregator) Events() []partybus.Event {
	return a.events
}
