package event

import (
	"github.com/wagoodman/canopy/cmd/canopy/internal"
	"github.com/wagoodman/go-partybus"
)

const (
	prefix        = internal.ApplicationName + "-"
	cliTypePrefix = prefix + "cli-"

	GoTestRunRequestType partybus.EventType = prefix + "go-test-run-request"
	GoTestType           partybus.EventType = prefix + "go-test-event"
	GoTestRunType        partybus.EventType = prefix + "go-test-run-event"

	CLIReport       partybus.EventType = cliTypePrefix + "report"
	CLINotification partybus.EventType = cliTypePrefix + "notification"
)
