// Copyright (c) 2014 Square, Inc

package osmain

import (
	"time"

	"github.com/square/inspect/metrics"
)

type darwinStats struct {
}

// RegisterOsSpecific registers OS dependent statistics
func registerOsSpecific(m *metrics.MetricContext, step time.Duration,
	osind *Stats) *darwinStats {
	x := new(darwinStats)
	return x
}

// PrintOsSpecific prints OS dependent statistics
func printOsSpecific(batchmode bool, layout *DisplayWidgets, v interface{}) {
}
