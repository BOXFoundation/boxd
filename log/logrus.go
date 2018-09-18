// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"github.com/heirko/go-contrib/logrusHelper"
	mate "github.com/heralight/logrus_mate"
	_ "github.com/heralight/logrus_mate/hooks/file"  // file log hook
	_ "github.com/heralight/logrus_mate/hooks/slack" // slack log hook
	"github.com/sirupsen/logrus"
)

type logrusLogger struct {
	logger *logrus.Logger
	tag    string
}

var _ Logger = (*logrusLogger)(nil)

var defaultLogrusLogger = logrus.New()

func init() {
	sourceHook := newHook()
	defaultLogrusLogger.AddHook(sourceHook)
}

// Setup logrus logger
func logrusSetup(cfg mate.LoggerConfig) {
	logrusHelper.SetConfig(
		defaultLogrusLogger,
		cfg,
	)
}

// newLogger creates a new logrus logger.
func logrusNewLogger(tag string) Logger {
	return &logrusLogger{
		logger: defaultLogrusLogger,
		tag:    tag,
	}
}

func (log *logrusLogger) entry() *logrus.Entry {
	return log.logger.WithFields(logrus.Fields{
		"tag": log.tag,
	})
}

// Debugf prints Debug level log
func (log *logrusLogger) Debugf(f string, v ...interface{}) {
	log.entry().Debugf(f, v...)
}

// Debug prints Debug level log
func (log *logrusLogger) Debug(v ...interface{}) {
	log.entry().Debug(v...)
}

// Infof prints Info level log
func (log *logrusLogger) Infof(f string, v ...interface{}) {
	log.entry().Infof(f, v...)
}

// Info prints Info level log
func (log *logrusLogger) Info(v ...interface{}) {
	log.entry().Info(v...)
}

// Warnf prints Warn level log
func (log *logrusLogger) Warnf(f string, v ...interface{}) {
	log.entry().Warnf(f, v...)
}

// Warn prints Warn level log
func (log *logrusLogger) Warn(v ...interface{}) {
	log.entry().Warn(v...)
}

// Errorf prints Error level log
func (log *logrusLogger) Errorf(f string, v ...interface{}) {
	log.entry().Errorf(f, v...)
}

// Error prints Error level log
func (log *logrusLogger) Error(v ...interface{}) {
	log.entry().Error(v...)
}

// Fatalf prints Fatal level log
func (log *logrusLogger) Fatalf(f string, v ...interface{}) {
	log.entry().Fatalf(f, v...)
}

// Fatal prints Fatal level log
func (log *logrusLogger) Fatal(v ...interface{}) {
	log.entry().Fatal(v...)
}

// Panicf prints Panic level log
func (log *logrusLogger) Panicf(f string, v ...interface{}) {
	log.entry().Panicf(f, v...)
}

// Panic prints Panic level log
func (log *logrusLogger) Panic(v ...interface{}) {
	log.entry().Panic(v...)
}
