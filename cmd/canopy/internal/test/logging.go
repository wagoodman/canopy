package test

import (
	"fmt"

	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"

	"github.com/anchore/go-logger"
)

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

func refFields(ref gotest.Reference, result ...gotest.Action) logger.Fields {
	f := logger.Fields{
		"name": fmt.Sprintf("%q", ref.String(false)),
	}
	if len(result) > 0 {
		f["result"] = result[0]
	}
	return f
}

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
