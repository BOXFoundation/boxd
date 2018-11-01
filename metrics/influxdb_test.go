// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metrics

import (
	"testing"

	"github.com/rcrowley/go-metrics"
)

func TestInfluxDb(t *testing.T) {
	registry := metrics.DefaultRegistry
	go collectSystemMetrics()
	InfluxDB(registry, interval, "http://localhost", 8086, "box", "", "")

}

