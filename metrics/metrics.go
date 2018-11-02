// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metrics

import (
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/BOXFoundation/boxd/log"
	"github.com/jbenet/goprocess"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/exp"
)

var logger = log.NewLogger("metrics")

const (
	interval = 2 * time.Second
	// MetricsEnabledFlag metrics enable flag
	MetricsEnabledFlag = "metrics"
)

var (
	enable = false
	quitCh chan (bool)
)

func init() {
	for _, arg := range os.Args {
		if strings.TrimLeft(arg, "-") == MetricsEnabledFlag {
			EnableMetrics()
			return
		}
	}
}

// EnableMetrics enable the metrics service
func EnableMetrics() {
	enable = true
	exp.Exp(metrics.DefaultRegistry)
}

// Run metrics monitor
func Run(config *Config, parent goprocess.Process) {
	if !enable {
		return
	}
	// collect sys metrics
	parent.Go(func(p goprocess.Process) {
		collectSystemMetrics()
	})
	// insert metrics data to influxdb
	parent.Go(func(p goprocess.Process) {
		NewInfluxDB(metrics.DefaultRegistry, interval, config.Host, config.Port, config.Db, config.User, config.Password, config.Tags)
	})
}

func collectSystemMetrics() {
	memstats := make([]*runtime.MemStats, 2)
	for i := 0; i < len(memstats); i++ {
		memstats[i] = new(runtime.MemStats)
	}

	allocs := metrics.GetOrRegisterMeter("system.allocs", nil)
	sys := metrics.GetOrRegisterMeter("system.sys", nil)
	frees := metrics.GetOrRegisterMeter("system.frees", nil)
	heapInuse := metrics.GetOrRegisterMeter("system.heapInuse", nil)
	stackInuse := metrics.GetOrRegisterMeter("system.stackInuse", nil)
	releases := metrics.GetOrRegisterMeter("system.release", nil)

	for i := 1; ; i++ {
		select {
		case <-quitCh:
			return
		default:
			runtime.ReadMemStats(memstats[i%2])
			allocs.Mark(int64(memstats[i%2].Alloc - memstats[(i-1)%2].Alloc))
			sys.Mark(int64(memstats[i%2].Sys - memstats[(i-1)%2].Sys))
			frees.Mark(int64(memstats[i%2].Frees - memstats[(i-1)%2].Frees))
			heapInuse.Mark(int64(memstats[i%2].HeapInuse - memstats[(i-1)%2].HeapInuse))
			stackInuse.Mark(int64(memstats[i%2].StackInuse - memstats[(i-1)%2].StackInuse))
			releases.Mark(int64(memstats[i%2].HeapReleased - memstats[(i-1)%2].HeapReleased))

			time.Sleep(2 * time.Second)
		}
	}

}

// Stop metrics monitor
func Stop() {
	quitCh <- true
}

// NewCounter create a new metrics Counter
func NewCounter(name string) metrics.Counter {
	if !enable {
		return new(metrics.NilCounter)
	}
	return metrics.GetOrRegisterCounter(name, metrics.DefaultRegistry)
}

// NewMeter create a new metrics Meter
func NewMeter(name string) metrics.Meter {
	if !enable {
		return new(metrics.NilMeter)
	}
	return metrics.GetOrRegisterMeter(name, metrics.DefaultRegistry)
}

// NewTimer create a new metrics Timer
func NewTimer(name string) metrics.Timer {
	if !enable {
		return new(metrics.NilTimer)
	}
	return metrics.GetOrRegisterTimer(name, metrics.DefaultRegistry)
}

// NewGauge create a new metrics Gauge
func NewGauge(name string) metrics.Gauge {
	if !enable {
		return new(metrics.NilGauge)
	}
	return metrics.GetOrRegisterGauge(name, metrics.DefaultRegistry)
}

// NewHistogramWithUniformSample create a new metrics History with Uniform Sample algorithm.
func NewHistogramWithUniformSample(name string, reservoirSize int) metrics.Histogram {
	if !enable {
		return new(metrics.NilHistogram)
	}
	return metrics.GetOrRegisterHistogram(name, nil, metrics.NewUniformSample(reservoirSize))
}
