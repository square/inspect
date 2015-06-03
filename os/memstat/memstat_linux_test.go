// Copyright (c) 2015 Square, Inc

package memstat

import (
	"testing"
	"time"

	"github.com/sorawee/inspect/metrics"
)

func TestMemstat(t *testing.T) {
	root = "testdata/t0/"
	// Initialize a metric context
	m := metrics.NewMetricContext("system")
	mstat := New(m, time.Millisecond*50)
	time.Sleep(time.Millisecond * 100)
	var expected float64 = 1.779400704e+09
	actual := mstat.Usage()
	if actual != expected {
		t.Errorf("Memstat Usage: %v expected: %v", actual, expected)
	}
}
