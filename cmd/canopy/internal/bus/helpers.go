package bus

import (
	"sync/atomic"

	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/clio"
)

// interrupted records whether the user interrupted the application (e.g. ctrl-c). The UI-driven interrupt is
// treated as a graceful exit by clio (the worker error is dropped), so we track it here for main to set a
// non-zero exit code.
var interrupted atomic.Bool

// Exit publishes a normal application exit event to the bus.
func Exit() {
	Publish(clio.ExitEvent(false))
}

// ExitWithInterrupt publishes an interrupt-based exit event to the bus.
// This indicates the application is exiting due to a user interrupt signal.
func ExitWithInterrupt() {
	MarkInterrupted()
	Publish(clio.ExitEvent(true))
}

// MarkInterrupted records that the user interrupted the application (e.g. via an OS signal), without
// publishing an exit event (used by the OS signal handler, which cancels the context directly).
func MarkInterrupted() {
	interrupted.Store(true)
}

// Interrupted reports whether the application was interrupted by the user (e.g. ctrl-c).
func Interrupted() bool {
	return interrupted.Load()
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
