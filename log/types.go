// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	mate "github.com/heralight/logrus_mate"
)

// Logger defines the box log functions
type Logger interface {
	SetLogLevel(level string)
	LogLevel() string
	Debugf(f string, v ...interface{})
	Debug(v ...interface{})
	Infof(f string, v ...interface{})
	Info(v ...interface{})
	Warnf(f string, v ...interface{})
	Warn(v ...interface{})
	Errorf(f string, v ...interface{})
	Error(v ...interface{})
	Fatalf(f string, v ...interface{})
	Fatal(v ...interface{})
	Panicf(f string, v ...interface{})
	Panic(v ...interface{})
}

// Config is the configuration of the logrus logger
type Config mate.LoggerConfig

// Setup loggers globally
func Setup(cfg *Config) {
	SetupLogrus(cfg)
}

// NewLogger creates a new logger.
func NewLogger(tag string) Logger {
	return NewLogrusLogger(tag)
}
