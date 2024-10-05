package ui

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/adapter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/gostd"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/jestsummary"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
	"github.com/wagoodman/go-partybus"
	"io"

	"github.com/anchore/clio"
)

func NewGoStdUI(testPkgs *golist.PackageCollection, json bool, verbose int, color bool) clio.UI {
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

	//reportWriter := os.Stdout
	//notificationWriter := os.Stderr
	reportReader, reportWriter := readerWriterPair()
	notificationReader, notificationWriter := readerWriterPair()

	switch {
	case json:
		handler = gostd.NewJSONHandler(reportWriter)
		writeToStderr = true
	case verbose > 0:
		handler = gostd.NewVerboseHandler(
			reportWriter,
			gostd.VerbosePackageConfig{
				PackageNameWidth:            maxPkgName,
				Color:                       color,
				IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
				HidePackagesWithNoTestFiles: true,
			},
		)
	default:
		handler = gostd.NewDefaultHandler(
			reportWriter,
			gostd.DefaultPackageConfig{
				PackageNameWidth:            maxPkgName,
				Color:                       color,
				IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
				HidePackagesWithNoTestFiles: true,
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
				Color:            color,
			}.New),
		)

	summaryHandler := jestsummary.NewFactory(
		presenter.JestTestResultSummaryConfig{
			Color:       color,
			ShowElapsed: true,
		},
	)

	c := NewTeaUIConfig().
		WithSimpleUI(ux).
		WithPrintReader(reportReader, notificationReader).
		WithFooter(summaryHandler)

	return NewTeaUI(c)
}

func readerWriterPair() (io.Reader, io.WriteCloser) {
	r, w := io.Pipe()
	return r, w
}
