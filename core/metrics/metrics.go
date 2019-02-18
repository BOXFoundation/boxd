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
	// MetricsUtxoSizeGauge records the size of utxos
	MetricsUtxoSizeGauge = metrics.NewGauge("box.chain.utxo.size")
	// MetricsBloomFilterGauge records the size of bloomfilter
	MetricsBloomFilterGauge = metrics.NewGauge("box.block.bloomfilter.size")

	// txpool metrics

	// MetricsTxPoolSizeGauge records the size of new block cache
	MetricsTxPoolSizeGauge = metrics.NewGauge("box.txpool.size")
	// MetricsOrphanTxPoolSizeGauge records the size of new block cache
	MetricsOrphanTxPoolSizeGauge = metrics.NewGauge("box.txpool.orphan_size")

	// memory details

	// MetricsMemHeapInuseGauge records the heap inused
	MetricsMemHeapInuseGauge = metrics.NewGauge("box.mem.heap.inuse")
	// MetricsMemHeapReleasedGauge records the heap released
	MetricsMemHeapReleasedGauge = metrics.NewGauge("box.mem.heap.released")
	// MetricsMemStackInuseGauge records the stack inused
	MetricsMemStackInuseGauge = metrics.NewGauge("box.mem.stack.inuse")
	// MetricsMemStackSysGauge records the stack allocated by sys
	MetricsMemStackSysGauge = metrics.NewGauge("box.mem.stack.sys")
)
