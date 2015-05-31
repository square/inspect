// Copyright (c) 2015 Square, Inc

package loadstat

import (
	"testing"
	"time"

	"github.com/square/inspect/metrics"
)

func TestLoadstat(t *testing.T) {
	root = "testdata/t0/"
	// Initialize a metric context
	m := metrics.NewMetricContext("system")
	lstat := New(m, time.Millisecond*50)
	time.Sleep(time.Millisecond * 1000)
	root = "testdata/t1/"
	time.Sleep(time.Millisecond * 100)
	var expected float64 = 0.13
	actual := lstat.OneMinute.Get()
	if actual != expected {
		t.Errorf("OneMinute loadavg: %v expected: %v", actual, expected)
	}
}
