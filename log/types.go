// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	mate "github.com/heralight/logrus_mate"
)

// Logger defines the box log functions
type Logger interface {
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

// Level can be Debug/Info/Warn/Error
type Level int

// Config is the configuration of the logrus logger
type Config mate.LoggerConfig

const (
	// LevelPanic enables panic level log
	LevelPanic Level = iota
	// LevelFatal enables fatal level log
	LevelFatal
	// LevelError enables error or higher level log
	LevelError
	// LevelWarn enables error or higher level log
	LevelWarn
	// LevelInfo enables error or higher level log
	LevelInfo
	// LevelDebug enables error or higher level log
	LevelDebug
)

// Setup loggers globally
func Setup(cfg *Config) {
	SetupLogrus(cfg)
}

// NewLogger creates a new logger.
func NewLogger(tag string) Logger {
	return NewLogrusLogger(tag)
}
