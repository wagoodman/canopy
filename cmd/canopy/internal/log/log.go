// Package log provides a singleton logger with automatic redaction support
// for sensitive values. It wraps anchore/go-logger with application-specific
// configuration and convenience functions.
package log

import (
	"github.com/anchore/go-logger"
	"github.com/anchore/go-logger/adapter/discard"
	"github.com/anchore/go-logger/adapter/redact"
)

var (
	// log is the singleton logger instance used throughout the application.
	// It defaults to a discard logger that drops all log messages until configured.
	log = discard.New()

	// store maintains the set of values that should be redacted from log output.
	store = redact.NewStore()
)

// Set configures the singleton logger instance with automatic redaction support.
// All subsequent logging calls will use this logger wrapped with the redaction store.
func Set(l logger.Logger) {
	log = redact.New(l, store)
}

// Get returns the current singleton logger instance.
func Get() logger.Logger {
	return log
}

// Redact adds sensitive values to the redaction store.
// These values will be automatically replaced in all log output.
func Redact(values ...string) {
	store.Add(values...)
}

// Errorf logs a formatted message at the error logging level.
func Errorf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}

// Error logs the given arguments at the error logging level.
func Error(args ...interface{}) {
	log.Error(args...)
}

// Warnf logs a formatted message at the warning logging level.
func Warnf(format string, args ...interface{}) {
	log.Warnf(format, args...)
}

// Warn logs the given arguments at the warning logging level.
func Warn(args ...interface{}) {
	log.Warn(args...)
}

// Infof logs a formatted message at the info logging level.
func Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}

// Info logs the given arguments at the info logging level.
func Info(args ...interface{}) {
	log.Info(args...)
}

// Debugf logs a formatted message at the debug logging level.
func Debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

// Debug logs the given arguments at the debug logging level.
func Debug(args ...interface{}) {
	log.Debug(args...)
}

// Tracef logs a formatted message at the trace logging level.
func Tracef(format string, args ...interface{}) {
	log.Tracef(format, args...)
}

// Trace logs the given arguments at the trace logging level.
func Trace(args ...interface{}) {
	log.Trace(args...)
}

// WithFields returns a message logger with multiple key-value fields attached.
// Fields should be provided as alternating key-value pairs.
func WithFields(fields ...interface{}) logger.MessageLogger {
	return log.WithFields(fields...)
}

// Nested returns a new logger with hard-coded key-value pairs.
// This is useful for creating contextual loggers for specific subsystems.
func Nested(fields ...interface{}) logger.Logger {
	return log.Nested(fields...)
}
