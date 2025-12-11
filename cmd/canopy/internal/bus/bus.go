// Package bus provides a singleton event bus for publishing test execution
// events throughout the application. It wraps go-partybus with application-specific
// event types and convenience functions.
package bus

import (
	"github.com/wagoodman/go-partybus"
)

var publisher partybus.Publisher

// Set configures the singleton event bus publisher used for all event publishing.
// This is optional; if no bus is provided, publish operations become no-ops.
// This allows the library to function gracefully whether or not event handling is configured.
func Set(p partybus.Publisher) {
	publisher = p
}

// publish sends an event onto the bus if one has been configured.
// If there is no bus set by the calling application, this does nothing.
func publish(e partybus.Event) {
	if publisher != nil {
		publisher.Publish(e)
	}
}
