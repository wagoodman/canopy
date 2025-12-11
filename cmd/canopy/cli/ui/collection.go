package ui

import (
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/clio"
)

var _ clio.UI = (*Collection)(nil)

// Collection multiplexes events to multiple UI implementations, allowing multiple output formats simultaneously.
// This enables scenarios like displaying a TUI while also writing JSON to a file.
type Collection struct {
	uis          []clio.UI
	subscription partybus.Unsubscribable
	lock         *sync.Mutex
}

// NewCollection creates a UI multiplexer that forwards events to all provided UI implementations.
func NewCollection(uis ...clio.UI) *Collection {
	return &Collection{
		uis:  uis,
		lock: &sync.Mutex{},
	}
}

// Setup initializes all UIs in the collection with the event bus subscription.
func (u *Collection) Setup(subscription partybus.Unsubscribable) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	return u.setup(subscription)
}

func (u *Collection) setup(subscription partybus.Unsubscribable) error {
	u.subscription = subscription
	var errs error
	for _, ui := range u.uis {
		if err := ui.Setup(subscription); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

// Handle forwards the event to all UIs in the collection, accumulating any errors.
func (u Collection) Handle(event partybus.Event) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	var errs error
	for _, ui := range u.uis {
		if err := ui.Handle(event); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

// Teardown shuts down all UIs in the collection, accumulating any errors.
func (u *Collection) Teardown(force bool) error {
	u.lock.Lock()
	defer u.lock.Unlock()
	var errs error
	for _, ui := range u.uis {
		if err := ui.Teardown(force); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}
