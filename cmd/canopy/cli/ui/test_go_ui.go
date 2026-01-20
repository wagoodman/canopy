package ui

import (
	"io"
	"os"
	"time"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/adapter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/gostd"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/gosummary"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/syncspinner"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/env"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"

	"github.com/anchore/clio"
)

// NewTestGoUI creates a new UI for displaying results in a similar format to the default go test output.
func NewTestGoUI(cfg TestUIConfig, maxPkgNameLength int) clio.UI {
	if cfg.IsTTY && cfg.Writer == nil {
		return newDynamicGoUI(cfg, maxPkgNameLength)
	}
	return newSafeGoUI(cfg, maxPkgNameLength)
}

func newDynamicGoUI(cfg TestUIConfig, maxPkgNameLength int) clio.UI {
	spin := syncspinner.New()

	common := state.Common{
		Spinner: spin.CurrentTick(),
	}

	reportReader, reportWriter := readerWriterPair()
	notificationReader, notificationWriter := readerWriterPair()

	loosePackageOrder := true               // allow the UI to skip ahead to packages that are taking a long time to complete
	stalePackageDuration := 3 * time.Second // this is the duration that a package can be stale before the UI skips ahead to the next package

	var h handler.Handler
	handlerPkgConfig := gostd.PackageConfig{
		PackageNameWidth:            maxPkgNameLength,
		Color:                       cfg.Color,
		IDE:                         ide.Select(&env.OSEnvironmentGetter{}),
		HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
		StripPackagePrefix:          cfg.StripPackagePrefix,
		LoosePackageOrder:           loosePackageOrder,
		StalePackageDuration:        stalePackageDuration,
		Grouping:                    cfg.Grouping,
	}

	if cfg.Verbose > 0 {
		h = gostd.NewVerboseHandler(reportWriter, handlerPkgConfig)
	} else {
		h = gostd.NewQuietHandler(reportWriter, handlerPkgConfig)
	}

	ux := newCoreUI().
		withNotifications().
		withReports().
		withHandlers(h).
		withStdout(reportWriter).
		withStderr(notificationWriter)

	summaryHandler := gosummary.NewFactory(
		presenter.DefaultGoTestResultSummaryConfig().
			WithColor(cfg.Color).
			WithPackageNameWidth(maxPkgNameLength).
			WithStripPackagePrefix(cfg.StripPackagePrefix).
			WithStalePackageDuration(stalePackageDuration).
			WithLoosePackageOrder(loosePackageOrder).
			WithCombineMultipleRuns(cfg.CombineMultipleRuns).
			WithDurationFromEvents(false), //  we're running with a true wall clock, so we want to use that. Otherwise you'll see the timers jitter, only updating when there is a test event that arrives.
		common,
	)

	c := NewTeaUIConfig().
		WithCoreUI(ux).
		WithSyncSpinner(spin).
		WithPrintReader(reportReader, notificationReader).
		WithFooter(summaryHandler)

	return NewTeaUI(c)
}

func newSafeGoUI(cfg TestUIConfig, maxPkgName int) clio.UI {
	var writeToStderr bool

	var reportWriter io.WriteCloser
	if cfg.Writer != nil {
		reportWriter = cfg.Writer
	} else {
		reportWriter = os.Stdout
	}
	notificationWriter := os.Stderr

	var h handler.Handler
	if cfg.Verbose > 0 {
		h = gostd.NewVerboseHandler(
			reportWriter,
			gostd.PackageConfig{
				PackageNameWidth:            maxPkgName,
				Color:                       cfg.Color,
				IDE:                         ide.Select(&env.OSEnvironmentGetter{}),
				HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
				Grouping:                    cfg.Grouping,
			},
		)
	} else {
		h = gostd.NewQuietHandler(
			reportWriter,
			gostd.PackageConfig{
				PackageNameWidth:            maxPkgName,
				Color:                       cfg.Color,
				IDE:                         ide.Select(&env.OSEnvironmentGetter{}),
				HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
				Grouping:                    cfg.Grouping,
			},
		)
	}

	ux := newCoreUI().
		withNotifications().
		withReports().
		withHandlers(h).
		withStdout(reportWriter).
		withStderr(notificationWriter).
		withHandledPresenters(
			adapter.NewTestRun(
				presenter.GoSummaryConfig{
					WriteToStderr:    writeToStderr,
					PackageNameWidth: maxPkgName,
					Color:            cfg.Color,
					// we're running with a true wall clock, so we want to use that. Otherwise you'll see the timers jitter,
					// only updating when there is a test event that arrives.
					DurationFromEvents:               false,
					ShowElapsedForRunningPackages:    true,
					ShowSummaryForUnrenderedPackages: true,
					ShowRunningTests:                 false, // it's safer to not thrash the number of lines we're writing to the terminal
					CombineMultipleRuns:              cfg.CombineMultipleRuns,
				}.New,
			),
		)

	return ux
}
