// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metrics

import (
	"github.com/BOXFoundation/boxd/metrics"
)

var (
	// block Metrics
	MetricsBlockHeightGauge         = metrics.NewGauge("box.block.height")
	MetricsBlockTailHashGauge       = metrics.NewGauge("box.block.tailhash")
	MetricsBlockOrphanPoolSizeGauge = metrics.NewGauge("box.block.orphanpool.size")
	MetricsBlockRevertMeter         = metrics.NewMeter("box.block.revert")
	MetricsBlockOnchainTimer        = metrics.NewTimer("box.block.onchain")
	MetricsBlockPackTxTime          = metrics.NewGauge("box.block.packtx")

	// txpool Metrics
	MetricsTxPoolSize       = metrics.NewGauge("box.txpool.size")
	MetricsOrphanTxPoolSize = metrics.NewGauge("box.txpool.orphan_size")
)
