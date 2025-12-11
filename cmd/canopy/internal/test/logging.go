package test

import (
	"fmt"

	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"

	"github.com/anchore/go-logger"
)

// logEvent logs a test event at an appropriate level based on the event action.
// Package-level events are logged at info level, individual tests at debug level.
// When logTestFailuresAsErrors is true, failures are logged as errors instead of info/debug.
func logEvent(e gotest.Event, logTestFailuresAsErrors bool) {
	switch e.Action {
	case gotest.StartAction:
		log.WithFields(refFields(e.Reference)).Info("running tests in package")
	case gotest.RunAction:
		log.WithFields(refFields(e.Reference)).Debug("running test case")
	case gotest.FailAction, gotest.PassAction, gotest.SkipAction:
		logTestResult(e, logTestFailuresAsErrors)
	}
}

// refFields creates log fields from a test reference and optional result action.
// Returns a logger.Fields map with the test reference name and optionally the result.
func refFields(ref gotest.Reference, result ...gotest.Action) logger.Fields {
	f := logger.Fields{
		"name": fmt.Sprintf("%q", ref.String(false)),
	}
	if len(result) > 0 {
		f["result"] = result[0]
	}
	return f
}

// logTestResult logs the final result of a test (pass, fail, or skip).
// Package-level results are logged at info, individual tests at debug.
// Failures can optionally be logged as errors based on the logTestFailuresAsErrors flag.
func logTestResult(e gotest.Event, logTestFailuresAsErrors bool) {
	switch e.Action {
	case gotest.FailAction:
		if logTestFailuresAsErrors {
			if e.Reference.IsPackage() {
				log.WithFields(refFields(e.Reference, e.Action)).Error("package tests completed")
			} else {
				log.WithFields(refFields(e.Reference, e.Action)).Error("test completed")
			}
		} else {
			if e.Reference.IsPackage() {
				log.WithFields(refFields(e.Reference, e.Action)).Info("package tests completed")
			} else {
				log.WithFields(refFields(e.Reference, e.Action)).Debug("test completed")
			}
		}
	case gotest.PassAction, gotest.SkipAction:
		if e.Reference.IsPackage() {
			log.WithFields(refFields(e.Reference, e.Action)).Info("package tests completed")
		} else {
			log.WithFields(refFields(e.Reference, e.Action)).Debug("test completed")
		}
	}
}
