// Copyright (c) 2014 Square, Inc

package cpustat

import (
	"testing"
	"time"

	"github.com/square/inspect/metrics"
)

func TestCPUUsage(t *testing.T) {
	root = "testdata/t0/"
	// Initialize a metric context
	m := metrics.NewMetricContext("system")
	cstat := New(m, time.Millisecond*50)
	time.Sleep(time.Millisecond * 1000)
	root = "testdata/t1/"
	time.Sleep(time.Millisecond * 100)
	var expected uint64 = 161584849
	actual := cstat.All.User.Get()
	if actual != expected {
		t.Errorf("CPU user counter: %v expected: %v", actual, expected)
	}
}
