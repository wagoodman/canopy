package ui

type Config struct {
	Color                    bool
	Verbose                  int
	ShowPackagesMissingTests bool
}

func DefaultConfig() Config {
	return Config{
		Color:                    true,
		Verbose:                  0,
		ShowPackagesMissingTests: false,
	}
}
