// Copyright (c) 2015 Square, Inc

// Package entropystat implements metrics collection for system entropy
package entropystat

import (
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/misc"
)

// to make testing easy
var path = "/proc/sys/kernel/random/entropy_avail"

// EntropyStat represents available entropy on the system
type EntropyStat struct {
	Available *metrics.Gauge
	m         *metrics.MetricContext
}

// New starts metrics collection every Step and registers with
// metricscontext
func New(m *metrics.MetricContext, Step time.Duration) *EntropyStat {
	stat := new(EntropyStat)
	stat.m = m
	// initialize all metrics and register them
	misc.InitializeMetrics(stat, m, "entropystat", true)
	// collect once
	stat.Collect()
	// collect metrics every Step
	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			stat.Collect()
		}
	}()
	return stat
}

// Collect populates Entropystat from /proc/sys/kernel/random/entropy_avail
func (stat *EntropyStat) Collect() {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	available, err := strconv.Atoi(strings.TrimSpace(string(file)))
	if err != nil {
		return
	}
	stat.Available.Set(float64(available))
}
