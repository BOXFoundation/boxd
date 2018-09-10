// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"io"
	l "log"
	"os"
	"sync"

	config "github.com/BOXFoundation/Quicksilver/config"
)

// Logger defines the box log functions
type Logger struct {
	Logger *l.Logger
	level  Level
}

// Level can be Debug/Info/Warn/Error
type Level int

// TODO Verbose level to enable spew dump (https://github.com/davecgh/go-spew)
const (
	LevelFatal Level = iota
	LevelError
	LevelWarn
	LevelInfo
	// LevelDebug debug level logs
	LevelDebug
)

// LevelValue is the map from name to value
var LevelValue = map[string]Level{
	"fatal": LevelFatal,
	"error": LevelError,
	"warn":  LevelWarn,
	"info":  LevelInfo,
	"debug": LevelDebug,
	"f":     LevelFatal,
	"e":     LevelError,
	"w":     LevelWarn,
	"i":     LevelInfo,
	"d":     LevelDebug,
}

// DefaultLevel is the default log level for new created logger
var defaultLevel Level
var defaultWriter io.Writer
var defaultFlags int
var allLoggers map[string]*Logger

var mutex sync.Mutex

func init() {
	defaultFlags = l.LstdFlags
	defaultWriter = os.Stdout
	defaultLevel = LevelDebug
	allLoggers = make(map[string]*Logger)
}

// Setup loggers globally
func Setup(cfg *config.Config) error {
	SetLevel(Level(cfg.LogLevel))

	return nil
}

// SetLevel sets the log level of all loggers
func SetLevel(level Level) {
	mutex.Lock()

	defaultLevel = level
	for tag, logger := range allLoggers {
		logger.Logger = l.New(defaultWriter, tag, defaultFlags)
	}

	mutex.Unlock()
}

// SetFlags sets the log flags
func SetFlags(flags int) {
	mutex.Lock()
	defaultFlags = flags
	for _, logger := range allLoggers {
		logger.Logger.SetFlags(defaultFlags)
	}
	mutex.Unlock()
}

// SetWriter updates all loggers' output writer.
func SetWriter(writer io.Writer) {
	mutex.Lock()
	defaultWriter = writer
	for tag, logger := range allLoggers {
		logger.Logger = l.New(defaultWriter, tag, defaultFlags)
	}
	mutex.Unlock()
}

// NewLogger creates a new logger.
func NewLogger(prefix string) *Logger {
	mutex.Lock()
	log, ok := allLoggers[prefix]
	if !ok {
		logger := l.New(defaultWriter, prefix, defaultFlags)
		log = &Logger{Logger: logger, level: defaultLevel}
	}
	mutex.Unlock()

	return log
}

// Debugf prints Debug level log
func (log *Logger) Debugf(f string, v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelDebug {
		log.Logger.Printf(f, v...)
	}
	mutex.Unlock()
}

// Debug prints Debug level log
func (log *Logger) Debug(v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelDebug {
		log.Logger.Print(v...)
	}
	mutex.Unlock()
}

// Infof prints Info level log
func (log *Logger) Infof(f string, v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelInfo {
		log.Logger.Printf(f, v...)
	}
	mutex.Unlock()
}

// Info prints Info level log
func (log *Logger) Info(v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelInfo {
		log.Logger.Print(v...)
	}
	mutex.Unlock()
}

// Warnf prints Warn level log
func (log *Logger) Warnf(f string, v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelWarn {
		log.Logger.Printf(f, v...)
	}
	mutex.Unlock()
}

// Warn prints Warn level log
func (log *Logger) Warn(v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelWarn {
		log.Logger.Print(v...)
	}
	mutex.Unlock()
}

// Errorf prints Error level log
func (log *Logger) Errorf(f string, v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelError {
		log.Logger.Printf(f, v...)
	}
	mutex.Unlock()
}

// Error prints Error level log
func (log *Logger) Error(v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelError {
		log.Logger.Print(v...)
	}
	mutex.Unlock()
}

// Fatalf prints Fatal level log
func (log *Logger) Fatalf(f string, v ...interface{}) {
	mutex.Lock()
	defer mutex.Unlock()

	if log.level >= LevelFatal {
		log.Logger.Fatalf(f, v...)
	}
}

// Fatal prints Fatal level log
func (log *Logger) Fatal(v ...interface{}) {
	mutex.Lock()
	defer mutex.Unlock()

	if log.level >= LevelFatal {
		log.Logger.Fatal(v...)
	}
}
