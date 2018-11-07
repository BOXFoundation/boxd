// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metrics

import (
	"testing"

	"github.com/rcrowley/go-metrics"
	influxdb "github.com/vrischmann/go-metrics-influxdb"
)

func TestInfluxDb(t *testing.T) {
	influxdb.InfluxDBWithTags(metrics.DefaultRegistry, interval, "http://localhost:8086", "box", "", "", nil)
}
