package parser

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/go-partybus"
)

// ErrBadPayload represents an error when an event payload doesn't match the expected structure.
type ErrBadPayload struct {
	// Type is the event type that had the bad payload.
	Type partybus.EventType

	// Field is the name of the field that was invalid.
	Field string

	// Value is the actual value that was encountered.
	Value interface{}
}

func (e *ErrBadPayload) Error() string {
	return fmt.Sprintf("event='%s' has bad event payload field='%v': '%+v'", string(e.Type), e.Field, e.Value)
}

// newPayloadErr creates a new ErrBadPayload error.
func newPayloadErr(t partybus.EventType, field string, value interface{}) error {
	return &ErrBadPayload{
		Type:  t,
		Field: field,
		Value: value,
	}
}

// checkEventType verifies that an event has the expected type.
func checkEventType(actual, expected partybus.EventType) error {
	if actual != expected {
		return newPayloadErr(expected, "Type", actual)
	}
	return nil
}

// ParseCLIReport extracts the context and report message from a CLIReport event.
// The context (Source field) is optional and may be empty.
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

// ParseCLINotification extracts the context and notification message from a CLINotification event.
// The context (Source field) is optional and may be empty.
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

// ParseGoTestType extracts a gotest.Event from a GoTestType event.
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

// ParseGoTestRunType extracts a gotest.Run from a GoTestRunType event.
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

// ParseGoTestRunRequestType extracts the runner configuration and session ID from a GoTestRunRequestType event.
func ParseGoTestRunRequestType(e partybus.Event) (*gotest.RunnerConfig, *uuid.UUID, error) {
	if err := checkEventType(e.Type, event.GoTestRunRequestType); err != nil {
		return nil, nil, err
	}

	obj, ok := e.Value.(gotest.RunnerConfig)
	if !ok {
		return nil, nil, newPayloadErr(e.Type, "Value", e.Value)
	}

	id, ok := e.Source.(uuid.UUID)
	if !ok {
		return nil, nil, newPayloadErr(e.Type, "Source", e.Source)
	}

	return &obj, &id, nil
}
