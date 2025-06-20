package ui

import (
	"io"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/adapter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/gostd"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/gostdref"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/gostdsummary"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/pkgframe"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/syncspinner"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/clio"
)

func NewGoStdUI(testPkgs *golist.PackageCollection, json bool, cfg Config) clio.UI {
	if json {
		return newJSONGoStdUI(cfg)
	}
	if cfg.IsTTY && cfg.Writer == nil {
		if cfg.Verbose > 0 {
			return newVerboseDynamicGoStdUI(testPkgs, cfg)
		}
		return newDefaultDynamicGoStdUI(testPkgs, cfg)
	}
	return newSafeGoStdUI(testPkgs, cfg)
}

func newVerboseDynamicGoStdUI(testPkgs *golist.PackageCollection, cfg Config) clio.UI {
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

	handler := gostd.NewVerboseHandler(
		reportWriter,
		gostd.VerbosePackageConfig{
			PackageNameWidth:            maxPkgName,
			Color:                       cfg.Color,
			IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
			HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
			HideExecutionTestEvents:     !cfg.ShowExecutionTestEvents,
		},
	)

	ux := newSimpleUI().
		withNotifications().
		withReports().
		withHandlers(handler).
		withStdout(reportWriter).
		withStderr(notificationWriter)

	summaryHandler := gostdsummary.NewFactory(
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

func readerWriterPair() (io.Reader, io.WriteCloser) {
	r, w := io.Pipe()
	return r, w
}

func newDefaultDynamicGoStdUI(testPkgs *golist.PackageCollection, cfg Config) clio.UI {
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

	pkgConfig := gostd.DefaultPackageConfig{
		PackageNameWidth:            maxPkgName,
		Color:                       cfg.Color,
		IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
		HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
	}

	sty := style.NewGoStd(cfg.Color)

	pkgModelFactory := func(e gotest.Event, common state.Common) tea.Model {
		return gostdref.NewModel(
			e.Reference.PackageRef(), // pass the package ref, not the exact test ref, as this will react to all package events
			common,
			func(ref gotest.Reference, common state.Common, completed map[gotest.Reference]struct{}, elapsed time.Duration) string {
				// show the package name, the number of completed tests, the elapsed time + the spinner
				return gostd.FormatPackageLine(common.Spinner.View, ref.Package, len(completed), []string{elapsed.String()}, "", sty, false, maxPkgName)
			},
			func(writer io.Writer, ref gotest.Reference) gostdref.Reactor {
				return gostd.NewDefaultPackage(writer, pkgConfig, ref)
			},
		)
	}

	common := state.Common{
		Spinner: spin.CurrentTick(),
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

			// we turn all of this off since pkgModelFactory will handle these details
			ShowRunningPackages: false,
			ShowRunningTests:    false,
			ShowRunningSubTests: false,
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
}

func newJSONGoStdUI(cfg Config) clio.UI {
	var handler partybus.Handler
	var reportWriter io.WriteCloser
	if cfg.Writer != nil {
		reportWriter = cfg.Writer
	} else {
		reportWriter = os.Stdout
	}

	handler = gostd.NewJSONHandler(reportWriter)
	ux := newSimpleUI().
		withNotifications().
		withReports().
		withHandlers(handler).
		withStdout(reportWriter)

	return ux
}

func newSafeGoStdUI(testPkgs *golist.PackageCollection, cfg Config) clio.UI {
	var handler partybus.Handler
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

	switch {
	case cfg.Verbose > 0:
		handler = gostd.NewVerboseHandler(
			reportWriter,
			gostd.VerbosePackageConfig{
				PackageNameWidth:            maxPkgName,
				Color:                       cfg.Color,
				IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
				HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
				HideExecutionTestEvents:     !cfg.ShowExecutionTestEvents,
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
