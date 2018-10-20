// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	ll "github.com/BOXFoundation/boxd/log/logrus"
	log "github.com/BOXFoundation/boxd/log/types"
)

var loggerMap = map[string]log.Logger{}

// Setup loggers globally
func Setup(cfg *log.Config) {
	log.Setup(ll.LoggerName, cfg)
}

// NewLogger creates a new logger.
func NewLogger(tag string) log.Logger {
	newLogger := log.NewLogger(ll.LoggerName, tag)
	if newLogger != nil {
		loggerMap[tag] = newLogger
	}
	return newLogger
}

// SetLogLevel sets all loggers log level
func SetLogLevel(newLevel string) (ok bool) {
	ok = true
	for _, logger := range loggerMap {
		originLevel := logger.LogLevel()
		logger.SetLogLevel(newLevel)
		currentLevel := logger.LogLevel()
		if currentLevel != newLevel {
			logger.Infof("Error setting log level from %s to %s", originLevel, newLevel)
			ok = false
		}
	}
	return
}
