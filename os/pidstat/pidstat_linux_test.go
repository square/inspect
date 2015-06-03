// Copyright (c) 2015 Square, Inc

package pidstat

import (
	"math"
	"testing"
	"time"

	"github.com/square/inspect/metrics"
)

func TestPidstatCPU(t *testing.T) {
	root = "testdata/t0/"
	// Initialize a metric context
	m := metrics.NewMetricContext("system")
	pstat := NewProcessStat(m, time.Millisecond*50)
	time.Sleep(time.Millisecond * 1000)
	root = "testdata/t1/"
	time.Sleep(time.Millisecond * 1000)
	root = "testdata/t2/"
	time.Sleep(time.Millisecond * 1000)
	root = "testdata/t3/"
	var expected float64 = 0.5
	actual := pstat.ByCPUUsage()[0].CPUUsage()
	if math.Abs(actual-expected) > 0.01 {
		t.Errorf("CPU usage for top pid: %v expected: %v", actual, expected)
	}
}

func TestPidstatMem(t *testing.T) {
	root = "testdata/t0/"
	// Initialize a metric context
	m := metrics.NewMetricContext("system")
	pstat := NewProcessStat(m, time.Millisecond*50)
	time.Sleep(time.Millisecond * 1000)
	root = "testdata/t1/"
	time.Sleep(time.Millisecond * 1000)
	var expected float64 = 1.794048e+06
	actual := pstat.ByMemUsage()[0].MemUsage()
	if math.Abs(actual-expected) > 0.01 {
		t.Errorf("Mem usage for top pid: %v expected: %v", actual, expected)
	}
}
