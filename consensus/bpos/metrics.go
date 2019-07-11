// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bpos

import (
	"github.com/BOXFoundation/boxd/metrics"
)

var (
	// MetricsMintTurnCounter signs whose turn to mint
	MetricsMintTurnCounter = metrics.NewCounter("box.bpos.mint.turn")
)
