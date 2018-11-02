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
	// MetricsBlockOnchainTimer        = metrics.NewTimer("box.block.onchain")
	// MetricsTxOnchainTimer        = metrics.NewTimer("box.transaction.onchain")
	// MetricsBlockPackTxTime          = metrics.NewGauge("box.block.packtx")

	// block_pool metrics

	// MetricsCachedBlockMsg records the size of new block cache
	MetricsCachedBlockMsg = metrics.NewGauge("box.block.new.cached")
	// MetricsLruCacheBlock records the size of lru cache
	MetricsLruCacheBlock = metrics.NewGauge("box.block.lru.cached")
	// metricsCachedDownloadBlock = metrics.NewGauge("box.block.download.cached")

	// metricsDuplicatedBlock   = metrics.NewCounter("box.block.duplicated")
	// metricsInvalidBlock      = metrics.NewCounter("box.block.invalid")
	// metricsTxsInBlock        = metrics.NewGauge("box.block.txs")
	// metricsBlockVerifiedTime = metrics.NewGauge("box.block.executed")
	// metricsTxVerifiedTime    = metrics.NewGauge("box.tx.executed")
	// metricsTxPackedCount     = metrics.NewGauge("box.tx.packed")
	// metricsTxUnpackedCount   = metrics.NewGauge("box.tx.unpacked")
	// metricsTxGivebackCount   = metrics.NewGauge("box.tx.giveback")

	// txpool metrics

	// MetricsTxPoolSize records the size of new block cache
	MetricsTxPoolSize                      = metrics.NewGauge("box.txpool.size")
	// MetricsOrphanTxPoolSize records the size of new block cache
	MetricsOrphanTxPoolSize                = metrics.NewGauge("box.txpool.orphan_size")
	// metricsReceivedTx                      = metrics.NewGauge("box.txpool.received")
	// metricsCachedTx                        = metrics.NewGauge("box.txpool.cached")
	// metricsBucketTx                        = metrics.NewGauge("box.txpool.bucket")
	// metricsCandidates                      = metrics.NewGauge("box.txpool.candidates")
	// metricsInvalidTx                       = metrics.NewCounter("box.txpool.invalid")
	// metricsDuplicateTx                     = metrics.NewCounter("box.txpool.duplicate")
	// metricsTxPoolBelowGasPrice             = metrics.NewCounter("box.txpool.below_gas_price")
	// metricsTxPoolOutOfGasLimit             = metrics.NewCounter("box.txpool.out_of_gas_limit")
	// metricsTxPoolGasLimitLessOrEqualToZero = metrics.NewCounter("box.txpool.gas_limit_less_equal_zero")

	// // transaction metrics
	// metricsTxSubmit     = metrics.NewMeter("box.transaction.submit")
	// metricsTxExecute    = metrics.NewMeter("box.transaction.execute")
	// metricsTxExeSuccess = metrics.NewMeter("box.transaction.execute.success")
	// metricsTxExeFailed  = metrics.NewMeter("box.transaction.execute.failed")

	// // event metrics
	// metricsCachedEvent = metrics.NewGauge("box.event.cached")

	// // unexpect behavior
	// metricsUnexpectedBehavior = metrics.NewGauge("box.unexpected")
)
