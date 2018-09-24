// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	ll "github.com/BOXFoundation/Quicksilver/log/logrus"
	log "github.com/BOXFoundation/Quicksilver/log/types"
)

// Setup loggers globally
func Setup(cfg *log.Config) {
	log.Setup(ll.LoggerName, cfg)
}

// NewLogger creates a new logger.
func NewLogger(tag string) log.Logger {
	return log.NewLogger(ll.LoggerName, tag)
}
