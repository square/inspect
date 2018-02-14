// Copyright (c) 2015 Square, Inc

package entropystat

import (
	"testing"
	"time"

	"github.com/square/inspect/metrics"
)

func TestEntropyStat(t *testing.T) {
	path = "testdata/entropy_avail"
	// Initialize a metric context
	m := metrics.NewMetricContext("system")
	stat := New(m, time.Millisecond*50)
	var expected = 14.0
	actual := stat.Available.Get()
	if actual != expected {
		t.Errorf("Available actual: %v expected: %v", actual, expected)
	}
}
