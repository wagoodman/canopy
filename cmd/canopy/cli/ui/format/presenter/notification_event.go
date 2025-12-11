package presenter

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"
)

var _ Presenter = (*NotificationEvent)(nil)

// NotificationEvent presents CLI notification events to stderr with highlighting.
type NotificationEvent struct {
	// event is the notification event to present.
	event partybus.Event
}

// NewNotificationEvent creates a presenter for CLI notification events.
func NewNotificationEvent(e partybus.Event) Presenter {
	return NotificationEvent{event: e}
}

// Present writes the notification message to stderr in magenta color.
func (p NotificationEvent) Present(_, stderr io.Writer) error {
	// 13 = high intensity magenta (ANSI 16 bit code)
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("13"))

	_, notification, err := parser.ParseCLINotification(p.event)
	if err != nil {
		return fmt.Errorf("failed to parse notification: %w", err)
	}

	if _, err := fmt.Fprintln(stderr, style.Render(notification)); err != nil {
		// don't let this be fatal
		log.WithFields("error", err).Warn("failed to write final notifications")
	}

	return nil
}
