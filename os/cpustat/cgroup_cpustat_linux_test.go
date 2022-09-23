package cpustat

import (
	"testing"
	"time"

	"github.com/square/inspect/metrics"
)

func TestCgroupLimits(t *testing.T) {
	m := metrics.NewMetricContext("system")
	c := NewCgroupStat(m, time.Millisecond*50)
	time.Sleep(time.Millisecond * 1000)
	c.Collect("testdata/t5")
	time.Sleep(time.Millisecond * 100)

	expectedLimits := map[string]float64{
		// This cgroup has a limit of -1, so it should inherit the
		// limit from its parent.
		"cpustat.cgroup.p2/aia130.sjc2b.square/traffic-exemplar/square-envoy.TotalCount": 2,
		"cpustat.cgroup.p2/aia130.sjc2b.square/traffic-exemplar.TotalCount":              2,
		"cpustat.cgroup.p2/aia130.sjc2b.square.TotalCount":                               6,
	}

	for k, expectedLimit := range expectedLimits {
		c := m.Gauges[k]
		if c == nil {
			t.Errorf("expected a limit metric for %s, did not find one", k)
			continue
		}
		actualLimit := c.Get()
		if actualLimit != expectedLimit {
			t.Errorf("%s: limit = %f, expected %f", k, actualLimit, expectedLimit)
		}
	}
}
