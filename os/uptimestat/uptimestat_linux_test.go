// Copyright (c) 2015 Square, Inc

package uptimestat

import (
	"testing"
	"time"

	"github.com/square/inspect/metrics"
)

func TestLoadstat(t *testing.T) {
	root = "testdata/t0/"
	// Initialize a metric context
	m := metrics.NewMetricContext("system")
	ustat := New(m, time.Millisecond*50)
	time.Sleep(time.Millisecond * 1000)
	root = "testdata/t1/"
	time.Sleep(time.Millisecond * 200)
	var expectedUp float64 = 1
	var expectedIdle float64 = 4
	actualUp := ustat.Uptime.Get()
	actualIdle := ustat.Idle.Get()
	if actualUp != expectedUp {
		t.Errorf("Uptime: %v expected: %v", actualUp, expectedUp)
	}
	if actualIdle != expectedIdle {
		t.Errorf("Uptime: %v expected: %v", actualIdle, expectedIdle)
	}
}
