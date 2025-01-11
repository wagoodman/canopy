package bus

import (
	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/clio"
)

func TestEvent(e gotest.Event) {
	publish(partybus.Event{
		Type:  event.GoTestType,
		Value: e,
	})
}

func TestRun(r gotest.Run) {
	publish(partybus.Event{
		Type:  event.GoTestRunType,
		Value: r,
	})
}

func TestRunRequest(id uuid.UUID, r gotest.RunnerConfig) {
	publish(partybus.Event{
		Type:   event.GoTestRunRequestType,
		Value:  r,
		Source: id,
	})
}

func Exit() {
	publish(clio.ExitEvent(false))
}

func ExitWithInterrupt() {
	publish(clio.ExitEvent(true))
}

func Report(report string) {
	publish(partybus.Event{
		Type:  event.CLIReport,
		Value: report,
	})
}

func Notify(message string) {
	publish(partybus.Event{
		Type:  event.CLINotification,
		Value: message,
	})
}
