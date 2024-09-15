package db

import (
	"context"
	"time"

	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"gorm.io/gorm/logger"
)

type logAdapter struct {
	info bool
}

func newLogger() logger.Interface {
	return logAdapter{}
}

func (l logAdapter) LogMode(logger.LogLevel) logger.Interface {
	return l
}

func (l logAdapter) Info(_ context.Context, fmt string, v ...interface{}) {
	if l.info {
		log.Infof("gorm: "+fmt, v...)
	}
}

func (l logAdapter) Warn(_ context.Context, fmt string, v ...interface{}) {
	log.Warnf("gorm: "+fmt, v...)
}

func (l logAdapter) Error(_ context.Context, fmt string, v ...interface{}) {
	log.Errorf("gorm: "+fmt, v...)
}

func (l logAdapter) Trace(_ context.Context, _ time.Time, _ func() (sql string, rowsAffected int64), _ error) {

}
