package gotest

import (
	"github.com/google/uuid"
)

type Run struct {
	ID uuid.UUID

	Config RunnerConfig
	Result Result
}

func NewRun(config RunnerConfig) *Run {
	return &Run{
		ID:     uuid.New(),
		Config: config,
	}
}
