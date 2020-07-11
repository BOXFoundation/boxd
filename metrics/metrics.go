// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metrics

import (
	"strings"
	"time"

	"github.com/BOXFoundation/boxd/log"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/exp"
	influxdb "github.com/vrischmann/go-metrics-influxdb"
)

var logger = log.NewLogger("metrics")

const (
	interval = 500 * time.Millisecond
)

func init() {
	exp.Exp(metrics.DefaultRegistry)
}

// Run metrics monitor
func Run(config *Config) {
	if !config.Enable {
		return
	}
	// insert metrics data to influxdb
	go func() {
		tags := make(map[string]string)
		for _, v := range config.Tags {
			values := strings.Split(v, ":")
			if len(values) != 2 {
				continue
			}
			tags[values[0]] = values[1]
		}
		influxdb.InfluxDBWithTags(metrics.DefaultRegistry, interval, config.Host, config.Db, config.User, config.Password, tags)
	}()
}

// NewCounter create a new metrics Counter
func NewCounter(name string) metrics.Counter {
	return metrics.GetOrRegisterCounter(name, metrics.DefaultRegistry)
}

// NewMeter create a new metrics Meter
func NewMeter(name string) metrics.Meter {
	return metrics.GetOrRegisterMeter(name, metrics.DefaultRegistry)
}

// NewTimer create a new metrics Timer
func NewTimer(name string) metrics.Timer {
	return metrics.GetOrRegisterTimer(name, metrics.DefaultRegistry)
}

// NewGauge create a new metrics Gauge
func NewGauge(name string) metrics.Gauge {
	return metrics.GetOrRegisterGauge(name, metrics.DefaultRegistry)
}

// NewHistogramWithUniformSample create a new metrics History with Uniform Sample algorithm.
func NewHistogramWithUniformSample(name string, reservoirSize int) metrics.Histogram {
	return metrics.GetOrRegisterHistogram(name, nil, metrics.NewUniformSample(reservoirSize))
}
