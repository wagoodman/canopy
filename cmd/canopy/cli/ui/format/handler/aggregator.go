package handler

import (
	"github.com/wagoodman/go-partybus"
)

var _ partybus.Handler = (*Aggregator)(nil)

// Aggregator is a handler that collects all events of a specific type for later processing.
type Aggregator struct {
	// ty is the event type to collect.
	ty partybus.EventType

	// events holds all collected events of the specified type.
	events []partybus.Event
}

// NewAggregator creates a new aggregator that collects events of the specified type.
func NewAggregator(ty partybus.EventType) *Aggregator {
	return &Aggregator{
		ty: ty,
	}
}

// Handle processes an event, storing it if it matches the configured event type.
func (a *Aggregator) Handle(e partybus.Event) error {
	if e.Type == a.ty {
		a.events = append(a.events, e)
	}
	return nil
}

// Events returns all events collected by this aggregator.
func (a Aggregator) Events() []partybus.Event {
	return a.events
}
