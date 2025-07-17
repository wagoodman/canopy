package ui

import "io"

type TestUIConfig struct {
	Color                   bool
	Verbose                 int
	ShowPackagesWithNoTests bool
	Writer                  io.WriteCloser
	IsTTY                   bool
}

func DefaultTestUIConfig() TestUIConfig {
	return TestUIConfig{
		Color:                   true,
		Verbose:                 0,
		ShowPackagesWithNoTests: false,
	}
}
