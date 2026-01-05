package bus

import (
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/clio"
)

// Exit publishes a normal application exit event to the bus.
func Exit() {
	Publish(clio.ExitEvent(false))
}

// ExitWithInterrupt publishes an interrupt-based exit event to the bus.
// This indicates the application is exiting due to a user interrupt signal.
func ExitWithInterrupt() {
	Publish(clio.ExitEvent(true))
}

// Report publishes a CLI report message event to the bus.
// Reports are typically longer-form output intended for user consumption.
func Report(report string) {
	Publish(partybus.Event{
		Type:  event.CLIReport,
		Value: report,
	})
}

// Notify publishes a CLI notification message event to the bus.
// Notifications are typically short status messages or alerts.
func Notify(message string) {
	Publish(partybus.Event{
		Type:  event.CLINotification,
		Value: message,
	})
}

func Print(message string) {
	Publish(partybus.Event{
		Type:  event.PrintType,
		Value: message + "\n",
	})
}
