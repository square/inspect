// Copyright (c) 2015 Square, Inc

package interfacestat

import (
	"testing"
	"time"

	"github.com/square/inspect/metrics"
)

func TestInterfaceStat(t *testing.T) {
	root = "testdata/t0/"
	// Initialize a metric context
	m := metrics.NewMetricContext("system")
	istat := New(m, time.Millisecond*50)
	time.Sleep(time.Millisecond * 1000)
	root = "testdata/t1/"
	time.Sleep(time.Millisecond * 100)
	var expected uint64 = 7070289382
	actual := istat.Interfaces["eth0"].Metrics.TXbytes.Get()
	if actual != expected {
		t.Errorf("interfacestat txbytes: %v expected: %v", actual, expected)
	}
}
