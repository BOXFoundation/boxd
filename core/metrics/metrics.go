// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metrics

import (
	"github.com/BOXFoundation/boxd/metrics"
)

var (
	// block Metrics

	// MetricsBlockHeightGauge records the hight of bc
	MetricsBlockHeightGauge = metrics.NewGauge("box.block.height")
	// MetricsBlockTailHashGauge records the tail hash of bc
	MetricsBlockTailHashGauge = metrics.NewGauge("box.block.tailhash")
	// MetricsTailBlockTxsSizeGauge records the size of txs
	MetricsTailBlockTxsSizeGauge = metrics.NewGauge("box.block.tail.txs.size")
	// MetricsBlockOrphanPoolSizeGauge records the size of orphan pool
	MetricsBlockOrphanPoolSizeGauge = metrics.NewGauge("box.block.orphanpool.size")
	// MetricsBlockRevertMeter records the bc revert times
	MetricsBlockRevertMeter = metrics.NewMeter("box.block.revert")

	// block_pool metrics

	// MetricsCachedBlockMsgGauge records the size of new block cache
	MetricsCachedBlockMsgGauge = metrics.NewGauge("box.block.new.cached")
	// MetricsLruCacheBlockGauge records the size of lru cache
	MetricsLruCacheBlockGauge = metrics.NewGauge("box.block.lru.cached")

	// txpool metrics

	// MetricsTxPoolSizeGauge records the size of new block cache
	MetricsTxPoolSizeGauge = metrics.NewGauge("box.txpool.size")
	// MetricsOrphanTxPoolSizeGauge records the size of new block cache
	MetricsOrphanTxPoolSizeGauge = metrics.NewGauge("box.txpool.orphan_size")
)
