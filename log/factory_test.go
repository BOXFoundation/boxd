// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import "testing"
import log "github.com/BOXFoundation/Quicksilver/log/types"

func TestLogrusInit(t *testing.T) {
	var logger = NewLogger("test")
	if logger == nil {
		t.Fatal("Get a nil logger.")
	}
}

func TestLogrusSetup(t *testing.T) {
	var logger = NewLogger("test")

	var levels = []string{
		"debug",
		"info",
		"warning",
		"error",
		"fatal",
	}

	for _, level := range levels {
		var cfg log.Config
		cfg.Level = level
		Setup(&cfg)
		if logger.LogLevel() != cfg.Level {
			t.Errorf("Invalid log level %s. It should be %s.", logger.LogLevel(), level)
		}
	}

	var oldLevel = logger.LogLevel()

	var cfg log.Config
	cfg.Level = "unknown"
	Setup(&cfg)
	if logger.LogLevel() != oldLevel {
		t.Errorf("Invalid log level %s. It should be %s.", logger.LogLevel(), oldLevel)
	}
}
