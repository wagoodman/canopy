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
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"

	"github.com/anchore/clio"
)

func NewGoUI(testPkgs *golist.PackageCollection, cfg Config) clio.UI {
	if cfg.IsTTY && cfg.Writer == nil {
		return newDynamicGoUI(testPkgs, cfg)
	}
	return newSafeGoUI(testPkgs, cfg)
}

func newDynamicGoUI(testPkgs *golist.PackageCollection, cfg Config) clio.UI { //nolint:funlen
	var pkgCount int
	maxPkgName := 30
	if testPkgs != nil {
		pkgs := testPkgs.Packages()
		for _, pkg := range pkgs {
			if len(pkg.ImportPath) > maxPkgName {
				maxPkgName = len(pkg.ImportPath)
			}
		}
		pkgCount = len(pkgs)
	}

	spin := syncspinner.New()

	common := state.Common{
		Spinner: spin.CurrentTick(),
	}

	reportReader, reportWriter := readerWriterPair()
	notificationReader, notificationWriter := readerWriterPair()

	loosePackageOrder := true               // allow the UI to skip ahead to packages that are taking a long time to complete
	stalePackageDuration := 3 * time.Second // this is the duration that a package can be stale before the UI skips ahead to the next package

	var h handler.Handler
	if cfg.Verbose > 0 {
		h = gostd.NewVerboseHandler(
			reportWriter,
			gostd.PackageConfig{
				PackageNameWidth:            maxPkgName,
				Color:                       cfg.Color,
				IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
				HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
				LoosePackageOrder:           loosePackageOrder,
				StalePackageDuration:        stalePackageDuration,
			},
		)
	} else {
		h = gostd.NewQuietHandler(
			reportWriter,
			gostd.PackageConfig{
				PackageNameWidth:            maxPkgName,
				Color:                       cfg.Color,
				IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
				HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
				LoosePackageOrder:           loosePackageOrder,
				StalePackageDuration:        stalePackageDuration,
			},
		)
	}

	ux := newSimpleUI().
		withNotifications().
		withReports().
		withHandlers(h).
		withStdout(reportWriter).
		withStderr(notificationWriter)

	summaryHandler := gosummary.NewFactory(
		presenter.DefaultGoTestResultSummaryConfig().
			WithColor(cfg.Color).WithPackageNameWidth(maxPkgName).
			WithPackageCount(pkgCount).
			WithStalePackageDuration(stalePackageDuration).
			WithLoosePackageOrder(loosePackageOrder).
			WithDurationFromEvents(false), //  we're running with a true wall clock, so we want to use that. Otherwise you'll see the timers jitter, only updating when there is a test event that arrives.
		common,
	)

	c := NewTeaUIConfig().
		WithSimpleUI(ux).
		WithSyncSpinner(spin).
		WithPrintReader(reportReader, notificationReader).
		WithFooter(summaryHandler)

	return NewTeaUI(c)
}

func newSafeGoUI(testPkgs *golist.PackageCollection, cfg Config) clio.UI {
	var writeToStderr bool
	var pkgCount int
	maxPkgName := 30
	if testPkgs != nil {
		pkgs := testPkgs.Packages()
		for _, pkg := range pkgs {
			if len(pkg.ImportPath) > maxPkgName {
				maxPkgName = len(pkg.ImportPath)
			}
		}
		pkgCount = len(pkgs)
	}

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
				IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
				HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
			},
		)
	} else {
		h = gostd.NewQuietHandler(
			reportWriter,
			gostd.PackageConfig{
				PackageNameWidth:            maxPkgName,
				Color:                       cfg.Color,
				IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
				HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
			},
		)
	}

	ux := newSimpleUI().
		withNotifications().
		withReports().
		withHandlers(h).
		withStdout(reportWriter).
		withStderr(notificationWriter).
		withHandledPresenters(
			adapter.NewTestRun(presenter.GoTestResultSummaryConfig{
				WriteToStderr:    writeToStderr,
				PackageNameWidth: maxPkgName,
				PackageCount:     pkgCount,
				Color:            cfg.Color,
				// we're running with a true wall clock, so we want to use that. Otherwise you'll see the timers jitter,
				// only updating when there is a test event that arrives.
				DurationFromEvents:               false,
				ShowElapsedForRunningPackages:    true,
				ShowSummaryForUnrenderedPackages: true,
				ShowRunningTests:                 false, // it's safer to not thrash the number of lines we're writing to the terminal
			}.New),
		)

	return ux
}
