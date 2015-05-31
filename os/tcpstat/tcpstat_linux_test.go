// Copyright (c) 2015 Square, Inc

package tcpstat

import (
	"testing"
	"time"

	"github.com/square/inspect/metrics"
)

func TestTcpstat(t *testing.T) {
	root = "testdata/t0/"
	// Initialize a metric context
	m := metrics.NewMetricContext("system")
	tstat := New(m, time.Millisecond*50)
	time.Sleep(time.Millisecond * 200)
	var expected float64 = 45
	actual := tstat.CurrEstab.Get()
	if actual != expected {
		t.Errorf("Tcpstat current estab: %v expected: %v", actual, expected)
	}
}
