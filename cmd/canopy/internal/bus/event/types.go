// Package event defines the event types and payloads used for inter-component
// communication via the event bus. It includes events for test execution,
// UI updates, and session management.
package event

import (
	"github.com/wagoodman/canopy/cmd/canopy/internal"
	"github.com/wagoodman/go-partybus"
)

const (
	prefix        = internal.ApplicationName + "-"
	cliTypePrefix = prefix + "cli-"

	// GoTestRunRequestType is published when a test run is requested.
	// Value: gotest.RunnerConfig, Source: uuid.UUID
	GoTestRunRequestType partybus.EventType = prefix + "go-test-run-request"

	// GoTestType is published for individual test events during execution.
	// Value: gotest.Event
	GoTestType partybus.EventType = prefix + "go-test-event"

	// GoTestRunType is published for test run lifecycle events.
	// Value: gotest.Run
	GoTestRunType partybus.EventType = prefix + "go-test-run-event"

	// PrintType is published for general print output messages.
	PrintType partybus.EventType = prefix + "print"

	// CLIReport is published for longer-form CLI output messages.
	// Value: string, Source: string (optional context)
	CLIReport partybus.EventType = cliTypePrefix + "report"

	// CLINotification is published for short CLI status messages.
	// Value: string, Source: string (optional context)
	CLINotification partybus.EventType = cliTypePrefix + "notification"
)
