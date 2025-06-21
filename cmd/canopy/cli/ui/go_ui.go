package ui

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/adapter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/gostd"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/gosummary"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/syncspinner"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
	"github.com/wagoodman/go-partybus"
	"io"
	"os"

	"github.com/anchore/clio"
)

func NewGoUI(testPkgs *golist.PackageCollection, cfg Config) clio.UI {
	if cfg.IsTTY && cfg.Writer == nil {
		return newDynamicGoUI(testPkgs, cfg)
	}
	return newSafeGoUI(testPkgs, cfg)
}

func newDynamicGoUI(testPkgs *golist.PackageCollection, cfg Config) clio.UI {
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

	var handler handler.Handler
	//if cfg.Verbose > 0 {
	//	handler = gostd.NewVerboseHandler(
	//		reportWriter,
	//		gostd.VerbosePackageConfig{
	//			PackageNameWidth:            maxPkgName,
	//			Color:                       cfg.Color,
	//			IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
	//			HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
	//			HideExecutionTestEvents:     false,
	//		},
	//	)
	//} else {
	handler = gostd.NewHandler(
		reportWriter,
		gostd.PackageConfig{
			PackageNameWidth:            maxPkgName,
			Color:                       cfg.Color,
			IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
			HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
		},
	)
	//}

	ux := newSimpleUI().
		withNotifications().
		withReports().
		withHandlers(handler).
		withStdout(reportWriter).
		withStderr(notificationWriter)

	summaryHandler := gosummary.NewFactory(
		presenter.GoStdTestResultSummaryConfig{
			Color:            cfg.Color,
			PackageNameWidth: maxPkgName,
			PackageCount:     pkgCount,
			HidePackageCount: true,
			//ShowElapsed: true,

			ShowRunningPackages: true,
			ShowRunningTests:    true,
			ShowRunningSubTests: true,
		},
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

	var handler partybus.Handler
	//if cfg.Verbose > 0 {
	//	handler = gostd.NewVerboseHandler(
	//		reportWriter,
	//		gostd.VerbosePackageConfig{
	//			PackageNameWidth:            maxPkgName,
	//			Color:                       cfg.Color,
	//			IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
	//			HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
	//			HideExecutionTestEvents:     false,
	//		},
	//	)
	//} else {
	handler = gostd.NewHandler(
		reportWriter,
		gostd.PackageConfig{
			PackageNameWidth:            maxPkgName,
			Color:                       cfg.Color,
			IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
			HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
		},
	)
	//}

	ux := newSimpleUI().
		withNotifications().
		withReports().
		withHandlers(handler).
		withStdout(reportWriter).
		withStderr(notificationWriter).
		withHandledPresenters(
			adapter.NewTestRun(presenter.GoStdTestResultSummaryConfig{
				WriteToStderr:    writeToStderr,
				PackageNameWidth: maxPkgName,
				PackageCount:     pkgCount,
				Color:            cfg.Color,
			}.New),
		)

	return ux
}
