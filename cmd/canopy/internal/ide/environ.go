package ide

import (
	"os"
	"strings"
)

// EnvironmentGetter represents the actions on the os package for retrieving environment variables.
type EnvironmentGetter interface {
	Getenv(key string) string
	LookupEnv(key string) (string, bool)
}

// OSEnvironmentGetter is a helper object that implements the EnvironmentGetter interface using the os package.
type OSEnvironmentGetter struct{}

// Getenv retrieves the value of the environment variable associated with the given key.
func (e *OSEnvironmentGetter) Getenv(key string) string {
	return os.Getenv(key)
}

// LookupEnv retrieves the value of the environment variable associated with the given key.
// The second return value indicates whether the variable is present in the environment.
func (e *OSEnvironmentGetter) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

type SnapshotEnvironmentGetter struct {
	Env map[string]string
}

func NewSnapshotEnvironmentGetter(state map[string]string) *SnapshotEnvironmentGetter {
	return &SnapshotEnvironmentGetter{
		Env: state,
	}
}

func NewSnapshotEnvironmentGetterFromOSEnv() *SnapshotEnvironmentGetter {
	env := make(map[string]string)
	state := os.Environ()
	for _, kv := range state {
		// split on the first '='
		parts := strings.SplitN(kv, "=", 2)
		env[parts[0]] = parts[1]
	}
	return &SnapshotEnvironmentGetter{
		Env: env,
	}
}

func (e *SnapshotEnvironmentGetter) Getenv(key string) string {
	return e.Env[key]
}

func (e *SnapshotEnvironmentGetter) LookupEnv(key string) (string, bool) {
	val, ok := e.Env[key]
	return val, ok
}
