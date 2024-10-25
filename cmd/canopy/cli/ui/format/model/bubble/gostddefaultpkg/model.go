package gostddefaultpkg

import (
	"bytes"
	"errors"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/gostd"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/bubbly/bubbles/frame"
)

var (
	_ tea.Model                = (*Model)(nil)
	_ frame.ImprintableElement = (*Model)(nil)
	_ frame.TerminalElement    = (*Model)(nil)
)

type Config struct {
	gostd.DefaultPackageConfig

	Style *style.GoStd
}

type Model struct {
	config Config
	ref    gotest.Reference
	action gotest.Action
	style  style.GoStd

	// event driven state
	common    state.Common
	start     *time.Time
	running   map[gotest.Reference]struct{}
	completed map[gotest.Reference]struct{}

	fragment *gostd.DefaultPackage
	buffer   *bytes.Buffer
}

func NewModel(ref gotest.Reference, common state.Common, config Config) *Model {
	stRef := config.Style
	if stRef == nil {
		st := style.NewGoStd(config.Color)
		stRef = &st
	}

	var buffer bytes.Buffer

	return &Model{
		config:    config,
		ref:       ref.PackageRef(),
		style:     *stRef,
		fragment:  gostd.NewDefaultPackage(&buffer, config.DefaultPackageConfig, ref),
		buffer:    &buffer,
		common:    common,
		running:   make(map[gotest.Reference]struct{}),
		completed: make(map[gotest.Reference]struct{}),
	}
}

func (j Model) Init() tea.Cmd {
	return nil
}

func (j Model) ShouldImprint() bool {
	return j.isExpired(true)
}

func (j Model) isExpired(enabled bool) bool {
	if !enabled {
		return false
	}
	switch j.action {
	case gotest.PassAction, gotest.FailAction, gotest.SkipAction:
		return true
	}
	return false
}

func (j Model) IsAlive() bool {
	return true
}

func (j Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	j.common.OnMessage(msg)
	switch msg := msg.(type) {
	case partybus.Event:
		return j.handlePartybusEvent(msg)
	}
	return j, nil
}

func (j Model) handlePartybusEvent(e partybus.Event) (tea.Model, tea.Cmd) {
	if e.Type != event.GoTestType {
		return j, nil
	}

	gt, err := parser.ParseGoTestType(e)
	if err != nil {
		log.WithFields("error", err).Error("unable to parse go test event")
		return j, nil
	}

	if gt.Reference.Package != j.ref.Package {
		return j, nil
	}

	j.trackRunningTests(gt)

	if j.start == nil {
		j.start = &gt.Time
	}

	if gt.Reference == j.ref {
		j.action = gt.Action
	}

	err = j.fragment.OnGoTestEvent(gt)
	switch {
	case err == nil:
		break
	case errors.Is(err, gostd.ErrPackageComplete):
		return j, nil
	default:
		panic("TODO: gostddefaultpkg error: " + err.Error())
	}

	return j, nil
}

func (j *Model) trackRunningTests(e gotest.Event) {
	if e.Reference.IsPackage() {
		return
	}
	switch e.Action {
	case gotest.RunAction:
		j.running[e.Reference] = struct{}{}
	case gotest.PassAction, gotest.FailAction, gotest.SkipAction:
		delete(j.running, e.Reference)
		j.completed[e.Reference] = struct{}{}
	}
}

func (j Model) View() string {
	if buffer := j.buffer.String(); buffer != "" {
		return strings.TrimSpace(buffer)
	}

	render := strings.TrimSpace(j.fragment.String())
	if render != "" {
		return render
	}

	var elapsed string
	if j.start != nil {
		elapsed = time.Since(*j.start).Truncate(time.Millisecond).String()

	}

	return gostd.FormatPackageLine(j.common.Spinner.View, j.ref.Package, len(j.completed), []string{elapsed}, "", j.style, false, j.config.PackageNameWidth)
}
