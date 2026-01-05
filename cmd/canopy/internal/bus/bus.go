// Package bus provides a singleton event bus for publishing test execution
// events throughout the application. It wraps go-partybus with application-specific
// event types and convenience functions.
package bus

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync/atomic"

	"github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/go-partybus"

	"github.com/anchore/go-logger"
)

var (
	publisher partybus.Publisher
	debug     = &atomic.Bool{}
)

func Debug(set bool) {
	debug.Store(set)
}

// Set configures the singleton event bus publisher used for all event publishing.
// This is optional; if no bus is provided, Publish operations become no-ops.
// This allows the library to function gracefully whether or not event handling is configured.
func Set(p partybus.Publisher) {
	publisher = p
}

// Publish sends an event onto the bus if one has been configured.
// If there is no bus set by the calling application, this does nothing.
func Publish(e partybus.Event) {
	if publisher != nil {
		if debug.Load() {
			//nolint:forbidigo
			if e.Error != nil {
				log.WithFields("error", e.Error).Errorf("bus Publish %q", e.Type)
			} else {
				var fields = logger.Fields{}
				if e.Source != nil {
					fields["source"] = formatValue(e.Source)
				}
				if e.Value != nil {
					if e.Type == event.GoTestRunType {
						fields["value"] = "redacted"
					} else {
						fields["value"] = formatValue(e.Value)
					}
				}

				log.WithFields(fields).Debugf("bus Publish %q", e.Type)
			}
		}
		publisher.Publish(e)
	}
}

// formatValue returns a pretty JSON string if the value is a struct type,
// otherwise formats using %v.
func formatValue(v any) string {
	if v == nil {
		return "<nil>"
	}

	rv := reflect.ValueOf(v)
	// dereference pointers to get the underlying type
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return "<nil>"
		}
		rv = rv.Elem()
	}

	if rv.Kind() == reflect.Struct {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}

	return fmt.Sprintf("%v", v)
}
