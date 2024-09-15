package ui

import (
	"os"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/adapter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/gostd"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
	"github.com/wagoodman/go-partybus"

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

	switch {
	case json:
		handler = gostd.NewJSONHandler(os.Stdout)
		writeToStderr = true
	case verbose > 0:
		handler = gostd.NewVerboseHandler(
			os.Stdout,
			gostd.VerbosePackageConfig{
				PackageNameWidth:            maxPkgName,
				Color:                       color,
				IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
				HidePackagesWithNoTestFiles: true,
			},
		)
	default:
		handler = gostd.NewDefaultHandler(
			os.Stdout,
			gostd.DefaultPackageConfig{
				PackageNameWidth:            maxPkgName,
				Color:                       color,
				IDE:                         ide.Select(&ide.OSEnvironmentGetter{}),
				HidePackagesWithNoTestFiles: true,
			},
		)
	}

	return newSimpleUI().
		withNotifications().
		withReports().
		withHandlers(handler).
		withHandledPresenters(
			adapter.NewTestRun(presenter.GoStdTestResultSummaryConfig{
				WriteToStderr:    writeToStderr,
				PackageNameWidth: maxPkgName,
				PackageCount:     pkgCount,
				Color:            color,
			}.New),
		)
}
