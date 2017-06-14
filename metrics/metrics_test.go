// Copyright (c) 2014 Square, Inc

package metrics

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	// Stop the global ticker used by counters to measure time. This forces time to be
	// advanced manually, making tests deterministic as long as they are sequential.
	ticker.Stop()
}

// advanceCounterTime advances counters' global clock by the given duration. Use this in
// unit tests to simulate the passage of time where you might be tempted to call
// time.Sleep().
func advanceCounterTime(t time.Duration) {
	atomic.AddInt64(&ticks, t.Nanoseconds())
}

func TestCounterRate(t *testing.T) {
	c := NewCounter()
	// simulate incrementing the counter every 10ms in two goroutines
	// rate ~ 200/sec
	for i := 0; i < 100; i++ {
		advanceCounterTime(time.Millisecond * 10)
		c.Add(1)
		c.Add(1)
	}

	want := 200.0
	out := c.ComputeRate()

	if math.Abs(want-out) > 10 {
		t.Errorf("c.ComputeRate() = %v, want %v", out, want)
	}
}

func TestCounterRateNoChange(t *testing.T) {
	c := NewCounter()
	c.Set(0)
	advanceCounterTime(time.Millisecond * 100)
	c.Set(0)
	want := 0.0
	out := c.ComputeRate()
	if math.IsNaN(out) || (math.Abs(out-want) > math.SmallestNonzeroFloat64) {
		t.Errorf("c.ComputeRate() = %v, want %v", out, want)
	}
}

func TestCounterRateOverflow(t *testing.T) {
	c := NewCounter()
	c.Set(0)
	advanceCounterTime(time.Millisecond * 100)
	c.Set(10)
	want := c.ComputeRate()
	t.Logf("Computed rate before reset %v", want)
	// counter reset
	c.Set(0)
	t.Logf("Counter state after reset=0 %v", c)
	out := c.ComputeRate()
	t.Logf("Computed rate after reset %v", out)
	if math.IsNaN(out) || (math.Abs(out-want) > math.SmallestNonzeroFloat64) {
		t.Errorf("c.ComputeRate() = %v, want %v", out, want)
	}
	advanceCounterTime(time.Millisecond * 1000)
	c.Set(1)
	t.Logf("Counter state after set=1 %v", c)
	advanceCounterTime(time.Millisecond * 1000)
	c.Set(2)
	t.Logf("Counter state after set=2 %v", c)
	want = 1.0
	out = c.ComputeRate()
	if math.IsNaN(out) || (math.Abs(out-want) > 0.05) {
		t.Errorf("c.ComputeRate() = %v, want %v", out, want)
	}
}

func TestDefaultGaugeVal(t *testing.T) {
	c := NewGauge()
	if !math.IsNaN(c.Get()) {
		t.Errorf("c.Get() = %v, want %v", c.Get(), math.NaN())
	}
}

func TestDefaultCounterVal(t *testing.T) {
	c := NewCounter()
	if c.Get() != 0 {
		t.Errorf("c.Get() = %v, want %v", c.Get(), 0)
	}
}

func TestBasicCounterVal(t *testing.T) {
	c := NewBasicCounter()
	if c.Get() != 0 {
		t.Errorf("c.Get() = %v, want %v", c.Get(), 0)
	}
}

func TestBasicCounterInc(t *testing.T) {
	c := NewBasicCounter()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Add(1)
		}()
	}

	// block till all goroutines finish
	wg.Wait()

	if c.Get() != 100 {
		t.Errorf("c.Get() = %v, want %v", c.Get(), 100)
	}
}

func TestStatsTimer(t *testing.T) {
	s := NewStatsTimer(time.Millisecond, 100) // keep 100 samples
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		x := i + 1
		go func() {
			defer wg.Done()
			stopWatch := s.Start()
			time.Sleep(time.Millisecond * time.Duration(x) * 10) // timers use the system clock
			s.Stop(stopWatch)
		}()
	}

	// block till all goroutines finish
	wg.Wait()

	pctile, err := s.Percentile(100)
	if math.Abs(pctile-1000) > 5 || err != nil {
		t.Errorf("Percentile expected: 1000 got: %v", pctile)
	}

	pctile, err = s.Percentile(75)
	if math.Abs(pctile-760) > 5 || err != nil {
		t.Errorf("Percentile expected: 750 got: %v", pctile)
	}
}
