// Copyright (c) 2015 Square, Inc

package netstat

import (
	"testing"
	"time"

	"github.com/square/inspect/metrics"
)

func init() { root = "testdata/t0/" }

func TestTcpstat(t *testing.T) {
	// Initialize a metric context
	m := metrics.NewMetricContext("system")
	tstat := New(m, time.Millisecond*50)
	time.Sleep(time.Millisecond * 200)
	var expected float64 = 45
	actual := tstat.TCPStat.CurrEstab.Get()
	if actual != expected {
		t.Errorf("Tcpstat current estab: %v expected: %v", actual, expected)
	}
}

func TestUDPStat(t *testing.T) {
	m := metrics.NewMetricContext("system")
	stat := New(m, time.Millisecond*50)
	time.Sleep(time.Millisecond * 200)
	actual := stat.UDPStat.OutDatagrams.Get()
	if actual != 248067 {
		t.Error("UDPStat OutDatagrams expected 248067 actual", actual)
	}
}
