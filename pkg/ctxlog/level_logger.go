package ctxlog

import (
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

type LevelLogger interface {
	log.Logger
	Debug(keyvals ...interface{})
	Info(keyvals ...interface{})
	Warn(keyvals ...interface{})
	Error(keyvals ...interface{})
}

type goKitLevelLogger struct {
	log.Logger
}

func (l goKitLevelLogger) Debug(keyvals ...interface{}) {
	level.Debug(l.Logger).Log(keyvals...)
}

func (l goKitLevelLogger) Info(keyvals ...interface{}) {
	level.Info(l.Logger).Log(keyvals...)
}

func (l goKitLevelLogger) Warn(keyvals ...interface{}) {
	level.Warn(l.Logger).Log(keyvals...)
}

func (l goKitLevelLogger) Error(keyvals ...interface{}) {
	level.Error(l.Logger).Log(keyvals...)
}
