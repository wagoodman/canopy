package parser

import (
	"fmt"

	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/go-partybus"
)

type ErrBadPayload struct {
	Type  partybus.EventType
	Field string
	Value interface{}
}

func (e *ErrBadPayload) Error() string {
	return fmt.Sprintf("event='%s' has bad event payload field='%v': '%+v'", string(e.Type), e.Field, e.Value)
}

func newPayloadErr(t partybus.EventType, field string, value interface{}) error {
	return &ErrBadPayload{
		Type:  t,
		Field: field,
		Value: value,
	}
}

func checkEventType(actual, expected partybus.EventType) error {
	if actual != expected {
		return newPayloadErr(expected, "Type", actual)
	}
	return nil
}

func ParseCLIReport(e partybus.Event) (string, string, error) {
	if err := checkEventType(e.Type, event.CLIReport); err != nil {
		return "", "", err
	}

	context, ok := e.Source.(string)
	if !ok {
		// this is optional
		context = ""
	}

	report, ok := e.Value.(string)
	if !ok {
		return "", "", newPayloadErr(e.Type, "Value", e.Value)
	}

	return context, report, nil
}

func ParseCLINotification(e partybus.Event) (string, string, error) {
	if err := checkEventType(e.Type, event.CLINotification); err != nil {
		return "", "", err
	}

	context, ok := e.Source.(string)
	if !ok {
		// this is optional
		context = ""
	}

	notification, ok := e.Value.(string)
	if !ok {
		return "", "", newPayloadErr(e.Type, "Value", e.Value)
	}

	return context, notification, nil
}

func ParseGoTestType(e partybus.Event) (gotest.Event, error) {
	if err := checkEventType(e.Type, event.GoTestType); err != nil {
		return gotest.Event{}, err
	}

	obj, ok := e.Value.(gotest.Event)
	if !ok {
		return gotest.Event{}, newPayloadErr(e.Type, "Value", e.Value)
	}

	return obj, nil
}

func ParseGoTestRunType(e partybus.Event) (*gotest.Run, error) {
	if err := checkEventType(e.Type, event.GoTestRunType); err != nil {
		return nil, err
	}

	r, ok := e.Value.(gotest.Run)
	if !ok {
		return nil, newPayloadErr(e.Type, "Value", e.Value)
	}

	return &r, nil
}
