package gotest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	mockReference1 = Reference{Package: "package1"}
	mockReference2 = Reference{Package: "package2"}
	mockReference3 = Reference{Package: "package3"}
	mockEventPass  = Event{Action: PassAction, Reference: mockReference1}
	mockEventFail  = Event{Action: FailAction, Reference: mockReference2}
	mockEventRun   = Event{Action: RunAction, Reference: mockReference3}
)

func TestNewResult(t *testing.T) {
	config := ResultConfig{
		TrackOtherOutput:   true,
		TrackFailingOutput: true,
	}
	result := NewResult(config)

	assert.NotNil(t, result)
	assert.Equal(t, config, result.config)
	assert.NotNil(t, result.references)
	assert.NotNil(t, result.testEventsByReference)
	assert.NotNil(t, result.testOutputByReference)
	assert.NotNil(t, result.referencesByAction)
	assert.NotNil(t, result.testReferencesByAction)
	assert.NotNil(t, result.conclusionEvent)
}

func TestResult_Update_WithTest_StartConditions(t *testing.T) {
	config := ResultConfig{}
	result := NewResult(config)

	passed, running := result.Passed()
	require.False(t, passed) // we've seen no events
	require.True(t, running)

	result.Update(mockEventRun)

	passed, running = result.Passed()
	require.True(t, passed) // we've seen an event, even though it's not a pass yet
	require.True(t, running)
}

func TestResult_Update_WithTest(t *testing.T) {
	config := ResultConfig{}
	result := NewResult(config)

	mockTestReference1 := Reference{Package: "package1", FuncName: "test1"}
	mockTestReference2 := Reference{Package: "package2", FuncName: "test2"}
	mockTestReference3 := Reference{Package: "package3", FuncName: "test3"}

	mockTestEventPass := Event{Action: PassAction, Reference: mockTestReference1}
	mockTestEventFail := Event{Action: FailAction, Reference: mockTestReference2}
	mockTestEventRun := Event{Action: RunAction, Reference: mockTestReference3}

	passed, running := result.Passed()
	require.False(t, passed) // we've seen no events, so it can't be a pass
	require.True(t, running)

	//update test with pass
	result.Update(mockTestEventPass)
	assert.Contains(t, result.References(), mockTestReference1)
	assert.Equal(t, []Event{mockTestEventPass}, result.ReferenceEvents(mockTestReference1))
	assert.Equal(t, []Reference{mockTestReference1}, result.ReferencesByAction(PassAction))
	assert.Equal(t, []Reference{mockTestReference1}, result.TestReferencesByAction(PassAction))
	assert.Equal(t, PassAction, result.ReferenceConclusiveAction(mockTestReference1))

	passed, running = result.Passed()
	require.True(t, passed)
	require.False(t, running) // TODO: is this right?

	// update with FailAction
	result.Update(mockTestEventFail)
	assert.Contains(t, result.References(), mockTestReference2)
	assert.Equal(t, []Event{mockTestEventFail}, result.ReferenceEvents(mockTestReference2))
	assert.Equal(t, []Reference{mockTestReference2}, result.ReferencesByAction(FailAction))
	assert.Equal(t, []Reference{mockTestReference2}, result.TestReferencesByAction(FailAction))
	assert.Equal(t, FailAction, result.ReferenceConclusiveAction(mockTestReference2))

	passed, running = result.Passed()
	require.False(t, passed)
	require.False(t, running) // TODO: is this right?

	// update with RunAction
	result.Update(mockTestEventRun)
	assert.Contains(t, result.References(), mockTestReference3)
	assert.Equal(t, []Event{mockTestEventRun}, result.ReferenceEvents(mockTestReference3))
	assert.Equal(t, []Reference{mockTestReference3}, result.ReferencesByAction(RunAction))
	assert.Equal(t, []Reference{mockTestReference3}, result.TestReferencesByAction(RunAction))

	passed, running = result.Passed()
	require.False(t, passed)
	require.True(t, running)
}

func TestResult_Update_WithPackage(t *testing.T) {
	config := ResultConfig{}
	result := NewResult(config)

	passed, running := result.Passed()
	require.False(t, passed) // we've seen no events, so it can't be a pass
	require.True(t, running)

	//update test with pass
	result.Update(mockEventPass)
	assert.Contains(t, result.References(), mockReference1)
	assert.Equal(t, []Event{mockEventPass}, result.ReferenceEvents(mockReference1))
	assert.Equal(t, []Reference{mockReference1}, result.ReferencesByAction(PassAction))
	assert.Equal(t, []Reference{}, result.TestReferencesByAction(PassAction)) // note the difference: no test was passed
	assert.Equal(t, PassAction, result.ReferenceConclusiveAction(mockReference1))

	passed, running = result.Passed()
	require.True(t, passed)
	require.True(t, running) // TODO: is this right?

	// update with fail
	result.Update(mockEventFail)
	assert.Contains(t, result.References(), mockReference2)
	assert.Equal(t, []Event{mockEventFail}, result.ReferenceEvents(mockReference2))
	assert.Equal(t, []Reference{mockReference2}, result.ReferencesByAction(FailAction))
	assert.Equal(t, []Reference{}, result.TestReferencesByAction(FailAction)) // note the difference: no test was passed
	assert.Equal(t, FailAction, result.ReferenceConclusiveAction(mockReference2))

	passed, running = result.Passed()
	require.False(t, passed)
	require.True(t, running) // TODO: is this right?

	// update with run
	result.Update(mockEventRun)
	assert.Contains(t, result.References(), mockReference3)
	assert.Equal(t, []Event{mockEventRun}, result.ReferenceEvents(mockReference3))
	assert.Equal(t, []Reference{mockReference3}, result.ReferencesByAction(RunAction))
	assert.Equal(t, []Reference{}, result.TestReferencesByAction(RunAction))
	assert.Equal(t, FailAction, result.ReferenceConclusiveAction(mockReference2)) // from before the run event

	passed, running = result.Passed()
	require.False(t, passed)
	require.True(t, running)
}

func TestResult_References(t *testing.T) {
	result := NewResult(ResultConfig{})
	result.references.Add(mockReference1)
	result.references.Add(mockReference2)

	references := result.References()
	assert.Contains(t, references, mockReference1)
	assert.Contains(t, references, mockReference2)
}

func TestResult_ReferenceEvents(t *testing.T) {
	result := NewResult(ResultConfig{})
	result.testEventsByReference[mockReference1] = []Event{mockEventPass}

	events := result.ReferenceEvents(mockReference1)
	assert.Equal(t, []Event{mockEventPass}, events)
}

func TestResult_ReferencesByAction(t *testing.T) {
	result := NewResult(ResultConfig{})
	result.referencesByAction[PassAction].Add(mockReference1)

	references := result.ReferencesByAction(PassAction)
	assert.Contains(t, references, mockReference1)
}

func TestResult_TestReferencesByAction(t *testing.T) {
	result := NewResult(ResultConfig{})
	result.testReferencesByAction[PassAction].Add(mockReference1)

	references := result.TestReferencesByAction(PassAction)
	assert.Contains(t, references, mockReference1)
}

func TestResult_TestStats(t *testing.T) {
	result := NewResult(ResultConfig{})
	result.testReferencesByAction[PassAction].Add(mockReference1)
	result.testReferencesByAction[FailAction].Add(mockReference2)
	result.testReferencesByAction[RunAction].Add(mockReference3)

	stats := result.TestStats()
	assert.Equal(t, 1, stats.Passed)
	assert.Equal(t, 1, stats.Failed)
	assert.Equal(t, 0, stats.Skipped)
	assert.Equal(t, 1, stats.Running)
}

func TestResult_ReferenceConclusiveAction(t *testing.T) {
	result := NewResult(ResultConfig{})
	result.conclusionEvent[mockReference1] = Event{
		Action: PassAction,
	}

	conclusion := result.ReferenceConclusiveAction(mockReference1)
	assert.Equal(t, PassAction, conclusion)
}

func TestResult_SetCoverage(t *testing.T) {
	result := NewResult(ResultConfig{})
	coverage := 75.0
	result.SetCoverage(&coverage)

	cov, ok := result.Coverage()
	assert.True(t, ok)
	assert.Equal(t, coverage, cov)

	result.SetCoverage(nil)
	cov, ok = result.Coverage()
	assert.False(t, ok)
	assert.Equal(t, 0.0, cov)
}
