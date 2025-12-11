package bus

import (
	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/clio"
)

// TestEvent publishes a go test event to the bus.
// This is used to broadcast individual test execution events (run, pass, fail, output, etc).
func TestEvent(e gotest.Event) {
	publish(partybus.Event{
		Type:  event.GoTestType,
		Value: e,
	})
}

// TestRun publishes a go test run event to the bus.
// This represents the overall status of a test run execution.
func TestRun(r gotest.Run) {
	publish(partybus.Event{
		Type:  event.GoTestRunType,
		Value: r,
	})
}

// TestRunRequest publishes a test run request event to the bus.
// The request includes a unique ID and the runner configuration to execute.
func TestRunRequest(id uuid.UUID, r gotest.RunnerConfig) {
	publish(partybus.Event{
		Type:   event.GoTestRunRequestType,
		Value:  r,
		Source: id,
	})
}

// Exit publishes a normal application exit event to the bus.
func Exit() {
	publish(clio.ExitEvent(false))
}

// ExitWithInterrupt publishes an interrupt-based exit event to the bus.
// This indicates the application is exiting due to a user interrupt signal.
func ExitWithInterrupt() {
	publish(clio.ExitEvent(true))
}

// Report publishes a CLI report message event to the bus.
// Reports are typically longer-form output intended for user consumption.
func Report(report string) {
	publish(partybus.Event{
		Type:  event.CLIReport,
		Value: report,
	})
}

// Notify publishes a CLI notification message event to the bus.
// Notifications are typically short status messages or alerts.
func Notify(message string) {
	publish(partybus.Event{
		Type:  event.CLINotification,
		Value: message,
	})
}
