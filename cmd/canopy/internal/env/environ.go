// Package env provides environment variable access abstractions.
package env

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

// SnapshotEnvironmentGetter provides an EnvironmentGetter implementation backed by a map snapshot.
type SnapshotEnvironmentGetter struct {
	Env map[string]string
}

// NewSnapshotEnvironmentGetter creates a new SnapshotEnvironmentGetter from the given state map.
func NewSnapshotEnvironmentGetter(state map[string]string) *SnapshotEnvironmentGetter {
	return &SnapshotEnvironmentGetter{
		Env: state,
	}
}

// NewSnapshotEnvironmentGetterFromOSEnv creates a new SnapshotEnvironmentGetter from the current OS environment.
func NewSnapshotEnvironmentGetterFromOSEnv() *SnapshotEnvironmentGetter {
	envMap := make(map[string]string)
	state := os.Environ()
	for _, kv := range state {
		// split on the first '='
		parts := strings.SplitN(kv, "=", 2)
		envMap[parts[0]] = parts[1]
	}
	return &SnapshotEnvironmentGetter{
		Env: envMap,
	}
}

// Getenv retrieves the value of the environment variable associated with the given key from the snapshot.
func (e *SnapshotEnvironmentGetter) Getenv(key string) string {
	return e.Env[key]
}

// LookupEnv retrieves the value of the environment variable associated with the given key from the snapshot.
// The second return value indicates whether the variable is present.
func (e *SnapshotEnvironmentGetter) LookupEnv(key string) (string, bool) {
	val, ok := e.Env[key]
	return val, ok
}

// Truthy returns true if the string value represents a truthy/positive value.
// Recognized truthy values (case-insensitive): "1", "t", "true", "yes", "y", "on"
func Truthy(val string) bool {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "1", "t", "true", "yes", "y", "on":
		return true
	}
	return false
}
