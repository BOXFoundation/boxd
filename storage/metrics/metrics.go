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
)
