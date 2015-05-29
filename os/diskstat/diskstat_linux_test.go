// Copyright (c) 2014 Square, Inc

package diskstat

import (
	"testing"
	"time"

	"github.com/square/inspect/metrics"
)

func TestDiskStat(t *testing.T) {
	root = "testdata/t0/"
	// Initialize a metric context
	m := metrics.NewMetricContext("system")
	dstat := New(m, time.Millisecond*50)
	time.Sleep(time.Millisecond * 1000)
	root = "testdata/t1/"
	time.Sleep(time.Millisecond * 100)
	var expected uint64 = 99609658
	actual := dstat.Disks["sda"].IOSpentMsecs.Get()
	if actual != expected {
		t.Errorf("Diskstat: %v expected: %v", actual, expected)
	}
}
