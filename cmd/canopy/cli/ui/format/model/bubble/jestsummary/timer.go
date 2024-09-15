package jestsummary

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type Timer struct {
	id       int
	duration time.Duration
}

type timerTickMessage struct {
	id int
}

var (
	lastTimerID int
	timerIDLock sync.Mutex
)

func nextTimerID() int {
	timerIDLock.Lock()
	defer timerIDLock.Unlock()
	lastTimerID++
	return lastTimerID
}

func newTimer(d time.Duration) Timer {
	return Timer{
		id:       nextTimerID(),
		duration: d,
	}
}

func (j Timer) tick() tea.Cmd {
	return tea.Tick(j.duration, func(_ time.Time) tea.Msg {
		return timerTickMessage{id: j.id}
	})
}
