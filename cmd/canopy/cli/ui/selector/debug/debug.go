package debug

import "sync"

var (
	value = "waiting for input..."
	lock  *sync.RWMutex
)

func init() {
	lock = &sync.RWMutex{}
}

func SetLine(s string) {
	lock.Lock()
	defer lock.Unlock()

	value = s
}

func Get() string {
	lock.RLock()
	defer lock.RUnlock()
	return value
}
