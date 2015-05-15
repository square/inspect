// Copyright (c) 2014 Square, Inc

package osmain

import (
	"github.com/square/inspect/metrics"
	"time"
)

type darwinStats struct {
}

// RegisterOsDependent registers OS dependent statistics
func RegisterOsDependent(
	m *metrics.MetricContext, step time.Duration,
	d *OsIndependentStats) *darwinStats {

	x := new(darwinStats)
	return x
}

// PrintOsDependent prints OS dependent statistics
func PrintOsDependent(d *darwinStats, batchmode bool) {
}
