package ui

import "io"

type Config struct {
	Color                   bool
	Verbose                 int
	ShowPackagesWithNoTests bool
	ShowStartTestEvents     bool
	Writer                  io.WriteCloser
	IsTTY                   bool
}

func DefaultConfig() Config {
	return Config{
		Color:                   true,
		Verbose:                 0,
		ShowPackagesWithNoTests: false,
		ShowStartTestEvents:     false,
	}
}
