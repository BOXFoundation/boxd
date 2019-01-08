// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

import (
	metrics "github.com/BOXFoundation/boxd/metrics"
)

var (
	metricsRevieveChSizeGauge = metrics.NewGauge("box.p2p.recieveCh.size")
	metricsPQTopChSizeGauge   = metrics.NewGauge("box.p2p.pq.top.size")
	metricsPQHighChSizeGauge  = metrics.NewGauge("box.p2p.pq.high.size")
	metricsPQMidChSizeGauge   = metrics.NewGauge("box.p2p.pq.mid.size")
	metricsPQLowChSizeGauge   = metrics.NewGauge("box.p2p.pq.low.size")

	metricsReadMeter  = metrics.NewMeter("box.p2p.read.request")
	metricsWriteMeter = metrics.NewMeter("box.p2p.write.request")
)
