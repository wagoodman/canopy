package ui

type Config struct {
	Color                   bool
	Verbose                 int
	ShowPackagesWithNoTests bool
}

func DefaultConfig() Config {
	return Config{
		Color:                   true,
		Verbose:                 0,
		ShowPackagesWithNoTests: false,
	}
}
