// Copyright (c) 2015 Square, Inc

// Package loadstat implements metrics collection related to loadavg
package loadstat

import (
	"bufio"
	"os"
	"strings"
	"time"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/misc"
)

type LoadStat struct {
	Metrics *LoadStatMetrics
	m       *metrics.MetricContext
}

func New(m *metrics.MetricContext, Step time.Duration) *LoadStat {
	s := new(LoadStat)
	s.Metrics = NewLoadStatMetrics(m, Step)
	return s
}

// Caution: reflection is used to read this struct to discover names
// Do not add new types
type LoadStatMetrics struct {
	OneMinute     *metrics.Gauge
	FiveMinute    *metrics.Gauge
	FifteenMinute *metrics.Gauge
	m             *metrics.MetricContext
}

func NewLoadStatMetrics(m *metrics.MetricContext, Step time.Duration) *LoadStatMetrics {
	c := new(LoadStatMetrics)
	c.m = m
	// initialize all metrics and register them
	misc.InitializeMetrics(c, m, "loadstat", true)
	// collect once
	c.Collect()
	// collect metrics every Step
	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			c.Collect()
		}
	}()
	return c
}

func (s *LoadStatMetrics) Collect() {
	file, err := os.Open("/proc/loadavg")
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := strings.Split(scanner.Text(), " ")
		if len(f) > 2 {
			s.OneMinute.Set(misc.ParseFloat(f[0]))
			s.FiveMinute.Set(misc.ParseFloat(f[1]))
			s.FifteenMinute.Set(misc.ParseFloat(f[2]))
		}
		break
	}
}
