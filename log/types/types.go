// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package types

import (
	"fmt"

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

type setupFunc func(*Config)
type newLoggerFunc func(string) Logger

// LoggerEntry is a logger impl entry
type LoggerEntry struct {
	Setup     setupFunc
	NewLogger newLoggerFunc
}

var loggers = map[string]*LoggerEntry{}

// Register registers a logger
func Register(name string, entry *LoggerEntry) {
	loggers[name] = entry
}

// Setup loggers globally
func Setup(name string, cfg *Config) {
	if entry, ok := loggers[name]; ok {
		entry.Setup(cfg)
	} else {
		fmt.Printf("Invalid logger: %s", name)
	}
}

// NewLogger creates a new logger.
func NewLogger(name, tag string) Logger {
	if entry, ok := loggers[name]; ok {
		return entry.NewLogger(tag)
	}

	fmt.Printf("Invalid logger: %s", name)
	return nil
}
