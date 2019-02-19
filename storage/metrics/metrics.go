// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metrics

import (
	"github.com/BOXFoundation/boxd/metrics"
)

var (
	// MetricsRocksdbCacheGauge records the base cache of default Options.
	MetricsRocksdbCacheGauge = metrics.NewGauge("box.rocksdb.cache.size")
	// MetricsRocksdbCacheCFGauge records the base cache of default Options.
	MetricsRocksdbCacheCFGauge = metrics.NewGauge("box.rocksdb.cache.CF.size")
	// MetricsRocksdbIdxFilterGauge records the Indexes and filter blocks.
	MetricsRocksdbIdxFilterGauge = metrics.NewGauge("box.rocksdb.idxfilter.size")
	// MetricsRocksdbMemtableGauge records the Memtable.
	MetricsRocksdbMemtableGauge = metrics.NewGauge("box.rocksdb.memtable.size")
	// MetricsRocksdbPinnedGauge records the Blocks pinned by iterators.
	MetricsRocksdbPinnedGauge = metrics.NewGauge("box.rocksdb.pinned.size")
)
