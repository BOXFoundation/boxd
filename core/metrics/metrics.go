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
	// MetricsBlockOrphanPoolSizeGauge records the size of orphan pool
	MetricsBlockOrphanPoolSizeGauge = metrics.NewGauge("box.block.orphanpool.size")
	// MetricsBlockRevertMeter records the bc revert times
	MetricsBlockRevertMeter = metrics.NewMeter("box.block.revert")

	// block_pool metrics

	// MetricsCachedBlockMsg records the size of new block cache
	MetricsCachedBlockMsg = metrics.NewGauge("box.block.new.cached")
	// MetricsLruCacheBlock records the size of lru cache
	MetricsLruCacheBlock = metrics.NewGauge("box.block.lru.cached")

	// txpool metrics

	// MetricsTxPoolSize records the size of new block cache
	MetricsTxPoolSize                      = metrics.NewGauge("box.txpool.size")
	// MetricsOrphanTxPoolSize records the size of new block cache
	MetricsOrphanTxPoolSize                = metrics.NewGauge("box.txpool.orphan_size")
)
