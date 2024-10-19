package options

import (
	"os"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/internal/log"

	"github.com/anchore/fangs"
)

var (
	_ fangs.PostLoader = (*Appearance)(nil)
)

type Appearance struct {
	NoColor                 bool `yaml:"no-color" json:"no-color" mapstructure:"no-color"`
	ShowPackagesWithNoTests bool `yaml:"show-packages-with-no-tests" json:"show-packages-with-no-tests" mapstructure:"show-packages-with-no-tests"`

	// tracker      *xflagset.Decorator
	//NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

func (t *Appearance) PostLoad() error {
	overrideNoColorFromEnv(&t.NoColor)
	return nil
}

func overrideNoColorFromEnv(opt *bool) {
	// override no-color with NO_COLOR env var
	noColorEnvVar := strings.TrimSpace(os.Getenv("NO_COLOR"))
	switch strings.ToLower(noColorEnvVar) {
	case "true", "1", "t":
		log.WithFields("NO_COLOR", noColorEnvVar).Trace("disabling colorized output")
		*opt = true
	}
}
