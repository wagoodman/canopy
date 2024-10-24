package ui

import (
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/gostd"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/gostddefaultpkg"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/gostdsummary"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/pkgframe"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/syncspinner"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
	"github.com/wagoodman/go-partybus"
	"golang.org/x/term"

	"github.com/anchore/clio"
)

func NewGoStdUI(testPkgs *golist.PackageCollection, json bool, cfg Config) clio.UI {
	if isATTY() && !json && cfg.Verbose == 0 {
		return newDynamicGoStdUI(testPkgs, cfg)
	}
	return newSafeGoStdUI(testPkgs, json, cfg)
}

func newDynamicGoStdUI(testPkgs *golist.PackageCollection, cfg Config) clio.UI {
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

	rowCfg := gostddefaultpkg.Config{
		DefaultPackageConfig: gostd.DefaultPackageConfig{
			PackageNameWidth:            maxPkgName,
			Color:                       cfg.Color,
			IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
			HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
		},
	}

	pkgModelFactory := func(e gotest.Event, common state.Common) tea.Model {
		return gostddefaultpkg.NewModel(e.Reference, common, rowCfg)
	}

	bodyHandler := pkgframe.NewFactory(
		pkgModelFactory,
		pkgframe.FactoryConfig{
			ShowPackagesMissingTests: cfg.ShowPackagesWithNoTests,
			Common:                   common,
		},
	)

	summaryHandler := gostdsummary.NewFactory(
		presenter.GoStdTestResultSummaryConfig{
			Color:            cfg.Color,
			PackageNameWidth: maxPkgName,
			PackageCount:     pkgCount,
			HidePackageCount: true,
			//ShowElapsed: true,
		},
		common,
	)

	c := NewTeaUIConfig(bodyHandler).
		WithSimpleUI(newSimpleUI().
			withNotifications().
			withReports(),
		).
		WithSyncSpinner(spin).
		WithFooter(summaryHandler)

	return NewTeaUI(c)

	////reportWriter := os.Stdout
	////notificationWriter := os.Stderr
	// reportReader, reportWriter := readerWriterPair()
	//notificationReader, notificationWriter := readerWriterPair()
	//
	//switch {
	//case json:
	//	handler = gostd.NewJSONHandler(reportWriter)
	//	//writeToStderr = true
	//case cfg.Verbose > 0:
	//	handler = gostd.NewVerboseHandler(
	//		reportWriter,
	//		gostd.VerbosePackageConfig{
	//			PackageNameWidth:            maxPkgName,
	//			Color:                       cfg.Color,
	//			IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
	//			HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
	//		},
	//	)
	//default:
	//	handler = gostd.NewDefaultHandler(
	//		reportWriter,
	//		gostd.DefaultPackageConfig{
	//			PackageNameWidth:            maxPkgName,
	//			Color:                       cfg.Color,
	//			IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
	//			HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
	//		},
	//	)
	//}
	//
	//ux := newSimpleUI().
	//	withNotifications().
	//	withReports().
	//	withHandlers(handler).
	//	withStdout(reportWriter).
	//	withStderr(notificationWriter)
	////withHandledPresenters(
	////	adapter.NewTestRun(presenter.GoStdTestResultSummaryConfig{
	////		WriteToStderr:    writeToStderr,
	////		PackageNameWidth: maxPkgName,
	////		PackageCount:     pkgCount,
	////		Color:            color,
	////	}.New),
	////)
	//
	//summaryHandler := gostdsummary.NewFactory(
	//	presenter.GoStdTestResultSummaryConfig{
	//		Color:            cfg.Color,
	//		PackageNameWidth: maxPkgName,
	//		PackageCount:     pkgCount,
	//		HidePackageCount: true,
	//		//ShowElapsed: true,
	//	},
	//)
	//
	//c := NewTeaUIConfig().
	//	WithSimpleUI(ux).
	//	WithPrintReader(reportReader, notificationReader).
	//	WithFooter(summaryHandler)
	//
	//return NewTeaUI(c)
}

func newSafeGoStdUI(testPkgs *golist.PackageCollection, json bool, cfg Config) clio.UI {
	var handler partybus.Handler
	// var writeToStderr bool
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

	// reportWriter := os.Stdout
	//notificationWriter := os.Stderr
	reportReader, reportWriter := readerWriterPair()
	notificationReader, notificationWriter := readerWriterPair()

	switch {
	case json:
		handler = gostd.NewJSONHandler(reportWriter)
		// writeToStderr = true
	case cfg.Verbose > 0:
		handler = gostd.NewVerboseHandler(
			reportWriter,
			gostd.VerbosePackageConfig{
				PackageNameWidth:            maxPkgName,
				Color:                       cfg.Color,
				IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
				HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
			},
		)
	default:
		handler = gostd.NewDefaultHandler(
			reportWriter,
			gostd.DefaultPackageConfig{
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
		withHandlers(handler).
		withStdout(reportWriter).
		withStderr(notificationWriter)
	// withHandledPresenters(
	//	adapter.NewTestRun(presenter.GoStdTestResultSummaryConfig{
	//		WriteToStderr:    writeToStderr,
	//		PackageNameWidth: maxPkgName,
	//		PackageCount:     pkgCount,
	//		Color:            color,
	//	}.New),
	//)

	spin := syncspinner.New()

	common := state.Common{
		Spinner: spin.CurrentTick(),
	}

	summaryHandler := gostdsummary.NewFactory(
		presenter.GoStdTestResultSummaryConfig{
			Color:            cfg.Color,
			PackageNameWidth: maxPkgName,
			PackageCount:     pkgCount,
			HidePackageCount: true,
			//ShowElapsed: true,
		},
		common,
	)

	c := NewTeaUIConfig().
		WithSimpleUI(ux).
		WithPrintReader(reportReader, notificationReader).
		WithSyncSpinner(spin).
		WithFooter(summaryHandler)

	return NewTeaUI(c)
}

func isATTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func readerWriterPair() (io.Reader, io.WriteCloser) {
	r, w := io.Pipe()
	return r, w
}
