// Copyright (c) 2015 Square, Inc

// Package uptimestat implements metrics collection related to loadavg
package uptimestat

import (
	"bufio"
	"os"
	"strings"
	"time"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/misc"
)

// to make testing easy
var root = "/"

// UptimeStat represents the two values found in /proc/uptime file
// current operating system.
// Caution: reflection is used to read this struct to discover names
// Do not add new types
type UptimeStat struct {
	// Total uptime
	Uptime *metrics.Gauge
	//Sum of idle time of all processors
	Idle *metrics.Gauge
	m    *metrics.MetricContext
}

// New starts metrics collection every Step and registers with
// metricscontext
func New(m *metrics.MetricContext, Step time.Duration) *UptimeStat {
	s := new(UptimeStat)
	s.m = m
	// initialize all metrics and register them
	misc.InitializeMetrics(s, m, "uptimestat", true)
	// collect once
	s.Collect()
	// collect metrics every Step
	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			s.Collect()
		}
	}()
	return s
}

// Collect populates Uptimestat by reading /proc/uptime
func (s *UptimeStat) Collect() {
	file, err := os.Open(root + "proc/uptime")
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := strings.Split(scanner.Text(), " ")
		if len(f) == 2 {
			s.Uptime.Set(misc.ParseFloat(f[0]))
			s.Idle.Set(misc.ParseFloat(f[1]))
		}
		break
	}
}
