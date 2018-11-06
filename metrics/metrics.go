// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metrics

import (
	"time"

	"github.com/BOXFoundation/boxd/log"
	"github.com/jbenet/goprocess"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/exp"
)

var logger = log.NewLogger("metrics")

const (
	interval = 2 * time.Second
)

func init() {
	exp.Exp(metrics.DefaultRegistry)
}

// Run metrics monitor
func Run(config *Config, parent goprocess.Process) {
	if !config.Enable {
		return
	}
	// insert metrics data to influxdb
	parent.Go(func(p goprocess.Process) {
		NewInfluxDB(metrics.DefaultRegistry, interval, config.Host, config.Port, config.Db, config.User, config.Password, config.Tags)
	})
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
