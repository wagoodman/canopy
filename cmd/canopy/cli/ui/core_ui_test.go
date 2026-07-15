package ui

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/go-partybus"

	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
)

// a handler whose String() closes a streaming group (here just a sentinel).
type stringerHandler struct{ out string }

func (h *stringerHandler) Handle(partybus.Event) error { return nil }
func (h *stringerHandler) String() string              { return h.out }

type writingPresenter struct{ out string }

func (p *writingPresenter) Present(stdout, _ io.Writer) error {
	_, err := io.WriteString(stdout, p.out)
	return err
}

// coreUI.Teardown must flush handler-buffered output (e.g. a closing ::endgroup::) before the
// presenters emit the summary, otherwise the summary is swallowed into a still-open CI group.
func TestCoreUITeardown_FlushesHandlersBeforePresenters(t *testing.T) {
	var buf bytes.Buffer
	n := &coreUI{
		stdout:     newNopWriteCloser(&buf),
		handlers:   []partybus.Handler{&stringerHandler{out: "::endgroup::\n"}},
		presenters: []presenter.Presenter{&writingPresenter{out: "PASS summary\n"}},
	}

	require.NoError(t, n.Teardown(false))
	require.Equal(t, "::endgroup::\nPASS summary\n", buf.String())
}
