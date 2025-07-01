package ui

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/adapter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/gostd"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/gosummary"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/syncspinner"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
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

	h := gostd.NewHandler(
		reportWriter,
		cfg.Verbose > 0,
		gostd.PackageConfig{
			PackageNameWidth:            maxPkgName,
			Color:                       cfg.Color,
			IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
			HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
		},
	)

	ux := newSimpleUI().
		withNotifications().
		withReports().
		withHandlers(h).
		withStdout(reportWriter).
		withStderr(notificationWriter)

	summaryHandler := gosummary.NewFactory(
		presenter.GoPPTestResultSummaryConfig{
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

	h := gostd.NewHandler(
		reportWriter,
		cfg.Verbose > 0,
		gostd.PackageConfig{
			PackageNameWidth:            maxPkgName,
			Color:                       cfg.Color,
			IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
			HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
		},
	)

	ux := newSimpleUI().
		withNotifications().
		withReports().
		withHandlers(h).
		withStdout(reportWriter).
		withStderr(notificationWriter).
		withHandledPresenters(
			adapter.NewTestRun(presenter.GoPPTestResultSummaryConfig{
				WriteToStderr:    writeToStderr,
				PackageNameWidth: maxPkgName,
				PackageCount:     pkgCount,
				Color:            cfg.Color,
			}.New),
		)

	return ux
}
