package adapter

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/presenter"
	"github.com/wagoodman/go-partybus"
)

type HandledPresenter interface {
	partybus.Handler
	presenter.Presenter
}
