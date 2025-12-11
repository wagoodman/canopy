// Package debug provides thread-safe storage for debug output displayed in the
// studio UI when debug mode is enabled. This is not meant to be used in production.
package debug

import "sync"

var (
	// value stores the current debug message.
	value = "waiting for input..."

	// lock protects concurrent access to value.
	lock *sync.RWMutex
)

func init() {
	lock = &sync.RWMutex{}
}

// SetLine updates the debug message to display. Safe for concurrent use.
func SetLine(s string) {
	lock.Lock()
	defer lock.Unlock()

	value = s
}

// Get retrieves the current debug message. Safe for concurrent use.
func Get() string {
	lock.RLock()
	defer lock.RUnlock()
	return value
}
