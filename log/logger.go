// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"io"
	l "log"
	"os"
	"strings"
	"sync"

	"github.com/spf13/viper"
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
}

// golang log impl
type gologger struct {
	logger *l.Logger
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
var allLoggers map[string]*gologger

var mutex sync.Mutex

func init() {
	defaultFlags = l.LstdFlags | l.Lshortfile
	defaultWriter = os.Stdout
	defaultLevel = LevelDebug
	allLoggers = make(map[string]*gologger)
}

// Setup loggers globally
func Setup(v *viper.Viper) {
	loglevel := strings.ToLower(v.GetString("log.level"))
	if loglevel, ok := LevelValue[loglevel]; ok {
		SetLevel(Level(loglevel))
	}
}

// SetLevel sets the log level of all loggers
func SetLevel(level Level) {
	mutex.Lock()

	defaultLevel = level
	for tag, logger := range allLoggers {
		logger.logger = l.New(defaultWriter, formatPrefix(tag), defaultFlags)
	}

	mutex.Unlock()
}

// SetFlags sets the log flags
func SetFlags(flags int) {
	mutex.Lock()
	defaultFlags = flags
	for _, logger := range allLoggers {
		logger.logger.SetFlags(defaultFlags)
	}
	mutex.Unlock()
}

// SetWriter updates all loggers' output writer.
func SetWriter(writer io.Writer) {
	mutex.Lock()
	defaultWriter = writer
	for tag, logger := range allLoggers {
		logger.logger = l.New(defaultWriter, formatPrefix(tag), defaultFlags)
	}
	mutex.Unlock()
}

// NewLogger creates a new logger.
func NewLogger(tag string) Logger {
	mutex.Lock()
	log, ok := allLoggers[tag]
	if !ok {
		logger := l.New(defaultWriter, formatPrefix(tag), defaultFlags)
		log = &gologger{logger: logger, level: defaultLevel}
	}
	mutex.Unlock()

	return log
}

func formatPrefix(tag string) string {
	return fmt.Sprintf("%s\t", tag)
}

// Debugf prints Debug level log
func (log *gologger) Debugf(f string, v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelDebug {
		log.logger.Output(2, log.sprintf(log.level, f, v...))
	}
	mutex.Unlock()
}

// Debug prints Debug level log
func (log *gologger) Debug(v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelDebug {
		log.logger.Output(2, log.sprint(log.level, v...))
	}
	mutex.Unlock()
}

// Infof prints Info level log
func (log *gologger) Infof(f string, v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelInfo {
		log.logger.Output(2, log.sprintf(log.level, f, v...))
	}
	mutex.Unlock()
}

// Info prints Info level log
func (log *gologger) Info(v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelInfo {
		log.logger.Output(2, log.sprint(log.level, v...))
	}
	mutex.Unlock()
}

// Warnf prints Warn level log
func (log *gologger) Warnf(f string, v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelWarn {
		log.logger.Output(2, log.sprintf(log.level, f, v...))
	}
	mutex.Unlock()
}

// Warn prints Warn level log
func (log *gologger) Warn(v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelWarn {
		log.logger.Output(2, log.sprint(log.level, v...))
	}
	mutex.Unlock()
}

// Errorf prints Error level log
func (log *gologger) Errorf(f string, v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelError {
		log.logger.Output(2, log.sprintf(log.level, f, v...))
	}
	mutex.Unlock()
}

// Error prints Error level log
func (log *gologger) Error(v ...interface{}) {
	mutex.Lock()
	if log.level >= LevelError {
		log.logger.Output(2, log.sprint(log.level, v...))
	}
	mutex.Unlock()
}

// Fatalf prints Fatal level log
func (log *gologger) Fatalf(f string, v ...interface{}) {
	mutex.Lock()
	defer mutex.Unlock()

	if log.level >= LevelFatal {
		log.logger.Output(2, log.sprintf(log.level, f, v...))
		os.Exit(1)
	}
}

// Fatal prints Fatal level log
func (log *gologger) Fatal(v ...interface{}) {
	mutex.Lock()
	defer mutex.Unlock()

	if log.level >= LevelFatal {
		log.logger.Output(2, log.sprint(log.level, v...))
		os.Exit(1)
	}
}

func (log *gologger) sprintf(level Level, f string, v ...interface{}) string {
	return fmt.Sprintf("%s\t%s", log.tag(level), fmt.Sprintf(f, v...))
}

func (log *gologger) sprint(level Level, v ...interface{}) string {

	return fmt.Sprintf("%s\t%s", log.tag(level), fmt.Sprint(v...))
}

func (log *gologger) tag(level Level) string {
	switch level {
	case LevelDebug:
		return "[D]"
	case LevelInfo:
		return "[I]"
	case LevelWarn:
		return "[W]"
	case LevelError:
		return "[E]"
	case LevelFatal:
		return "[F]"
	default:
		return "[*]"
	}
}
