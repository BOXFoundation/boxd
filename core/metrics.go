// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package core

import (
	metrics "github.com/BOXFoundation/boxd/metrics"
)

// Metrics for core
var (
	// block metrics
	metricsBlockHeightGauge      = metrics.NewGauge("neb.block.height")
	metricsBlocktailHashGauge    = metrics.NewGauge("neb.block.tailhash")
	metricsBlockRevertTimesGauge = metrics.NewGauge("neb.block.revertcount")
	metricsBlockRevertMeter      = metrics.NewMeter("neb.block.revert")
	metricsBlockOnchainTimer     = metrics.NewTimer("neb.block.onchain")
	metricsTxOnchainTimer        = metrics.NewTimer("neb.transaction.onchain")
	metricsBlockPackTxTime       = metrics.NewGauge("neb.block.packtx")

	// block_pool metrics
	metricsCachedNewBlock      = metrics.NewGauge("neb.block.new.cached")
	metricsCachedDownloadBlock = metrics.NewGauge("neb.block.download.cached")
	metricsLruPoolCacheBlock   = metrics.NewGauge("neb.block.lru.poolcached")
	metricsLruCacheBlock       = metrics.NewGauge("neb.block.lru.blocks")
	metricsLruTailBlock        = metrics.NewGauge("neb.block.lru.tailblock")

	metricsDuplicatedBlock   = metrics.NewCounter("neb.block.duplicated")
	metricsInvalidBlock      = metrics.NewCounter("neb.block.invalid")
	metricsTxsInBlock        = metrics.NewGauge("neb.block.txs")
	metricsBlockVerifiedTime = metrics.NewGauge("neb.block.executed")
	metricsTxVerifiedTime    = metrics.NewGauge("neb.tx.executed")
	metricsTxPackedCount     = metrics.NewGauge("neb.tx.packed")
	metricsTxUnpackedCount   = metrics.NewGauge("neb.tx.unpacked")
	metricsTxGivebackCount   = metrics.NewGauge("neb.tx.giveback")

	// txpool metrics
	metricsReceivedTx                      = metrics.NewGauge("neb.txpool.received")
	metricsCachedTx                        = metrics.NewGauge("neb.txpool.cached")
	metricsBucketTx                        = metrics.NewGauge("neb.txpool.bucket")
	metricsCandidates                      = metrics.NewGauge("neb.txpool.candidates")
	metricsInvalidTx                       = metrics.NewCounter("neb.txpool.invalid")
	metricsDuplicateTx                     = metrics.NewCounter("neb.txpool.duplicate")
	metricsTxPoolBelowGasPrice             = metrics.NewCounter("neb.txpool.below_gas_price")
	metricsTxPoolOutOfGasLimit             = metrics.NewCounter("neb.txpool.out_of_gas_limit")
	metricsTxPoolGasLimitLessOrEqualToZero = metrics.NewCounter("neb.txpool.gas_limit_less_equal_zero")

	// transaction metrics
	metricsTxSubmit     = metrics.NewMeter("neb.transaction.submit")
	metricsTxExecute    = metrics.NewMeter("neb.transaction.execute")
	metricsTxExeSuccess = metrics.NewMeter("neb.transaction.execute.success")
	metricsTxExeFailed  = metrics.NewMeter("neb.transaction.execute.failed")

	// event metrics
	metricsCachedEvent = metrics.NewGauge("neb.event.cached")

	// unexpect behavior
	metricsUnexpectedBehavior = metrics.NewGauge("neb.unexpected")
)

