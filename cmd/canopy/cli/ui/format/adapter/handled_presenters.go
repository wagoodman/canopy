package adapter

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/go-partybus"
)

// HandledPresenter combines event handling with presentation capabilities,
// allowing types to both process events and format output.
type HandledPresenter interface {
	partybus.Handler
	presenter.Presenter
}
