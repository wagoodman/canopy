package gotest

import (
	"time"

	"github.com/google/uuid"
)

type Run struct {
	ID uuid.UUID

	Start  time.Time
	End    *time.Time
	Config RunnerConfig
	Result Result
}

func NewRun(config RunnerConfig) *Run {
	return &Run{
		ID:     uuid.New(),
		Config: config,
	}
}

func (r Run) Elapsed() time.Duration {
	if r.End == nil {
		return time.Since(r.Start)
	}
	return r.End.Sub(r.Start)
}
