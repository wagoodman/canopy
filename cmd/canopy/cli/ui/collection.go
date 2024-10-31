package ui

import (
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/clio"
)

var _ clio.UI = (*Collection)(nil)

type Collection struct {
	uis          []clio.UI
	subscription partybus.Unsubscribable
	lock         *sync.Mutex
}

func NewCollection(uis ...clio.UI) *Collection {
	return &Collection{
		uis:  uis,
		lock: &sync.Mutex{},
	}
}

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
