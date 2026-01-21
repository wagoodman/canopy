package ui

import (
	"io"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/adapter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/goxx"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/goref"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/gosummary"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/pkgframe"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/syncspinner"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/env"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/clio"
)

// NewTestGoxxUI creates a new UI for displaying Go test results using the goxx handler.
// This is less stable than the default UI, and should be used with caution (will probably be removed in the future).
func NewTestGoxxUI(cfg TestUIConfig, maxPkgName int) clio.UI {
	if cfg.IsTTY && cfg.Writer == nil {
		if cfg.Verbose > 0 {
			return newVerboseDynamicGoxxUI(cfg, maxPkgName)
		}
		return newDefaultDynamicGoxxUI(cfg, maxPkgName)
	}
	return newSafeGoxxUI(cfg, maxPkgName)
}

func newVerboseDynamicGoxxUI(cfg TestUIConfig, maxPkgName int) clio.UI {
	spin := syncspinner.New()

	common := state.Common{
		Spinner: spin.CurrentTick(),
	}

	reportReader, reportWriter := readerWriterPair()
	notificationReader, notificationWriter := readerWriterPair()

	handler := goxx.NewVerboseHandler(
		reportWriter,
		goxx.VerbosePackageConfig{
			PackageNameWidth:            maxPkgName,
			Color:                       cfg.Color,
			IDE:                         ide.Select(&env.OSEnvironmentGetter{}),
			HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
			ExecutionMarkers:            cfg.ExecutionMarkers,
		},
	)

	ux := newCoreUI().
		withNotifications().
		withReports().
		withHandlers(handler).
		withStdout(reportWriter).
		withStderr(notificationWriter)

	summaryHandler := gosummary.NewFactory(
		presenter.GoSummaryConfig{
			Color:            cfg.Color,
			PackageNameWidth: maxPkgName,
			ShowRunningTests: true,
		},
		common,
	)

	c := NewTeaUIConfig().
		WithCoreUI(ux).
		WithSyncSpinner(spin).
		WithPrintReader(reportReader, notificationReader).
		WithFooter(summaryHandler)

	return NewTeaUI(c)
}

func readerWriterPair() (io.Reader, io.WriteCloser) {
	r, w := io.Pipe()
	return r, w
}

func newDefaultDynamicGoxxUI(cfg TestUIConfig, maxPkgName int) clio.UI {
	spin := syncspinner.New()

	pkgConfig := goxx.QuietPackageConfig{
		PackageNameWidth:            maxPkgName,
		Color:                       cfg.Color,
		IDE:                         ide.Select(&env.OSEnvironmentGetter{}),
		HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
		ExecutionMarkers:            cfg.ExecutionMarkers,
	}

	sty := style.NewGo(cfg.Color)

	pkgModelFactory := func(e gotest.Event, common state.Common) tea.Model {
		return goref.NewModel(
			e.Reference.PackageRef(), // pass the package ref, not the exact test ref, as this will react to all package events
			common,
			func(ref gotest.Reference, common state.Common, completed map[gotest.Reference]struct{}, elapsed time.Duration) string {
				// show the package name, the number of completed tests, the elapsed time + the spinner
				return presenter.Package{
					Status:         common.Spinner.View,
					Name:           ref.Package,
					TestsCompleted: len(completed),
					Aux:            []string{elapsed.String()},
					Trailer:        "",
					Style:          sty,
					FormatStatus:   false,
					MaxTestName:    maxPkgName,
				}.String()
			},
			func(writer io.Writer, ref gotest.Reference) goref.Reactor {
				return goxx.NewQuietPackage(writer, pkgConfig, ref)
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

	summaryHandler := gosummary.NewFactory(
		presenter.GoSummaryConfig{
			Color:            cfg.Color,
			PackageNameWidth: maxPkgName,
			// we turn all of this off since pkgModelFactory will handle these details
			ShowRunningTests: false,
		},
		common,
	)

	c := NewTeaUIConfig(bodyHandler).
		WithCoreUI(newCoreUI().
			withNotifications().
			withReports(),
		).
		WithSyncSpinner(spin).
		WithFooter(summaryHandler)

	return NewTeaUI(c)
}

func newSafeGoxxUI(cfg TestUIConfig, maxPkgName int) clio.UI {
	var handler partybus.Handler
	var writeToStderr bool

	var reportWriter io.WriteCloser
	if cfg.Writer != nil {
		reportWriter = cfg.Writer
	} else {
		reportWriter = os.Stdout
	}
	notificationWriter := os.Stderr

	switch {
	case cfg.Verbose > 0:
		handler = goxx.NewVerboseHandler(
			reportWriter,
			goxx.VerbosePackageConfig{
				PackageNameWidth:            maxPkgName,
				Color:                       cfg.Color,
				IDE:                         ide.Select(&env.OSEnvironmentGetter{}),
				HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
				ExecutionMarkers:            cfg.ExecutionMarkers,
			},
		)
	default:
		handler = goxx.NewQuietHandler(
			reportWriter,
			goxx.QuietPackageConfig{
				PackageNameWidth:            maxPkgName,
				Color:                       cfg.Color,
				IDE:                         ide.Select(&env.OSEnvironmentGetter{}),
				HidePackagesWithNoTestFiles: !cfg.ShowPackagesWithNoTests,
				ExecutionMarkers:            cfg.ExecutionMarkers,
			},
		)
	}

	ux := newCoreUI().
		withNotifications().
		withReports().
		withHandlers(handler).
		withStdout(reportWriter).
		withStderr(notificationWriter).
		withHandledPresenters(
			adapter.NewTestRun(presenter.GoSummaryConfig{
				WriteToStderr:    writeToStderr,
				PackageNameWidth: maxPkgName,
				Color:            cfg.Color,
			}.New),
		)

	return ux
}
