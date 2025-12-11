package db

import (
	"context"
	"time"

	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"gorm.io/gorm/logger"
)

// logAdapter adapts the canopy internal logger to the GORM logger interface.
// It bridges GORM's database logging to the application's logging system.
type logAdapter struct {
	// info controls whether informational messages are logged (currently disabled by default).
	info bool
}

// newLogger creates a new GORM logger adapter using the canopy internal logger.
func newLogger() logger.Interface {
	return logAdapter{}
}

// LogMode creates a new logger instance with the specified log level.
// This implementation currently ignores the level parameter.
func (l logAdapter) LogMode(logger.LogLevel) logger.Interface {
	return l
}

// Info logs an informational message from GORM operations.
// Messages are only logged if the info field is true.
func (l logAdapter) Info(_ context.Context, fmt string, v ...interface{}) {
	if l.info {
		log.Infof("gorm: "+fmt, v...)
	}
}

// Warn logs a warning message from GORM operations.
func (l logAdapter) Warn(_ context.Context, fmt string, v ...interface{}) {
	log.Warnf("gorm: "+fmt, v...)
}

// Error logs an error message from GORM operations.
func (l logAdapter) Error(_ context.Context, fmt string, v ...interface{}) {
	log.Errorf("gorm: "+fmt, v...)
}

// Trace logs SQL trace information from GORM operations.
// This implementation is a no-op to avoid excessive logging.
func (l logAdapter) Trace(_ context.Context, _ time.Time, _ func() (sql string, rowsAffected int64), _ error) {

}
