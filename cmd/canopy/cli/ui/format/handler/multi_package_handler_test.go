package handler

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/go-partybus"
)

type mockHandler struct {
	mock.Mock
}

func (m *mockHandler) Handle(e partybus.Event) error {
	args := m.Called(e)
	return args.Error(0)
}

func (m *mockHandler) OnGoTestEvent(event gotest.Event) error {
	args := m.Called(event)
	return args.Error(0)
}

func (m *mockHandler) String() string {
	args := m.Called()
	return args.String(0)
}

func TestNewMultiPackageHandler(t *testing.T) {
	mockFactory := func(ref gotest.Reference, writer io.Writer) Handler {
		return &mockHandler{}
	}
	h := NewMultiPackageHandler(mockFactory)

	require.NotNil(t, h, "Expected non-nil handler")
	assert.IsType(t, &multiPackageHandler{}, h, "Expected handler to be of type *multiPackageHandler")
}

func TestMultiPackageHandler_Handle(t *testing.T) {
	mh := new(mockHandler)
	mockWriter := new(bytes.Buffer)
	mockFactory := func(ref gotest.Reference, writer io.Writer) Handler {
		return mh
	}

	m := &multiPackageHandler{
		packages: make(map[string]Handler),
		factory:  mockFactory,
		writer:   mockWriter,
	}

	parsedEvent := gotest.Event{Reference: gotest.Reference{Package: "mypackage", FuncName: "mytest"}}

	testEvent := partybus.Event{
		Type:  event.GoTestType,
		Value: parsedEvent,
	}

	mh.On("OnGoTestEvent", parsedEvent).Return(nil)

	err := m.Handle(testEvent)
	assert.NoError(t, err, "Expected no error from Handle")
	require.Len(t, m.packages, 1, "Expected 1 package in packages map")
	assert.Equal(t, "mypackage", m.order[0], "Expected package 'mypackage' in order")
	mh.AssertCalled(t, "OnGoTestEvent", parsedEvent)
}

func TestMultiPackageHandler_OnGoTestEvent(t *testing.T) {
	mh := new(mockHandler)
	mockWriter := new(bytes.Buffer)
	mockFactory := func(ref gotest.Reference, writer io.Writer) Handler {
		return mh
	}

	m := &multiPackageHandler{
		packages: make(map[string]Handler),
		factory:  mockFactory,
		writer:   mockWriter,
	}

	goTestEvent := gotest.Event{
		Reference: gotest.Reference{
			Package: "mypackage",
		},
	}

	mh.On("OnGoTestEvent", goTestEvent).Return(nil)

	err := m.OnGoTestEvent(goTestEvent)
	assert.NoError(t, err, "Expected no error from OnGoTestEvent")
	require.Len(t, m.packages, 1, "Expected 1 package in packages map")
	assert.Equal(t, "mypackage", m.order[0], "Expected package 'mypackage' in order")
	mh.AssertCalled(t, "OnGoTestEvent", goTestEvent)
}

func TestMultiPackageHandler_String(t *testing.T) {
	mockHandler1 := new(mockHandler)
	mockHandler2 := new(mockHandler)

	mockHandler1.On("String").Return("Handler1Output")
	mockHandler2.On("String").Return("Handler2Output")

	m := &multiPackageHandler{
		packages: map[string]Handler{
			"package1": mockHandler1,
			"package2": mockHandler2,
		},
		order: []string{"package1", "package2"},
	}

	output := m.String()

	expectedOutput := "Handler1OutputHandler2Output"
	assert.Equal(t, expectedOutput, output, "Expected concatenated output of all handlers")
	mockHandler1.AssertCalled(t, "String")
	mockHandler2.AssertCalled(t, "String")
}
