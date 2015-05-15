// Copyright (c) 2014 Square, Inc

package metrics

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Counters
type Counter struct {
	v       uint64
	p       uint64
	rate    float64
	ticks_p int64
	ticks_v int64
	mu      sync.RWMutex
}

// Counters differ from BasicCounter by having additional
// fields for computing rate. Operations on counter hold
// a mutex. use BasicCounter if you need lock-free counters 
func NewCounter() *Counter {
	c := new(Counter)
	c.Reset()
	return c
}

// Reset() - resets all internal variables to defaults
// Usually called from NewCounter but useful if you have to
// re-use and existing object
func (c *Counter) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.rate = 0.0
	c.ticks_p = 0
	c.ticks_v = 0
	c.v = 0
	c.p = 0
}

// Set Counter value. This is useful if you are reading a metric
// that is already a counter
func (c *Counter) Set(v uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ticks_v = TICKS
	atomic.StoreUint64(&c.v, v)

	// baseline for rate calculation
	if c.ticks_p == 0 {
		c.p = c.v
		c.ticks_p = c.ticks_v
	}
}

// Add value to counter
func (c *Counter) Add(delta uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ticks_v = TICKS
	atomic.AddUint64(&c.v, delta)

	// baseline for rate calculation
	if c.ticks_p == 0 {
		c.p = c.v
		c.ticks_p = c.ticks_v
	}
}

// Get value of counter
func (c *Counter) Get() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.v
}

// ComputeRate() calculates the rate of change of counter per
// second. (acquires a lock)
// Since we avoid locking on Set/Add operations, rate can be
// inaccurate on highly contended threads

func (c *Counter) ComputeRate() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	rate := 0.0

	delta_t := c.ticks_v - c.ticks_p
	delta_v := c.v - c.p

	// we have two samples, compute rate and
	// cache it away
	if delta_t > 0 && c.v >= c.p {
		rate = (float64(delta_v) / float64(delta_t)) * NS_IN_SEC
		// update baseline
		c.p = c.v
		c.ticks_p = c.ticks_v
		// cache rate calculated
		c.rate = rate
	}

	return c.rate
}

// MarshalJSON returns a byte slice of JSON representation of
// counter
func (c *Counter) MarshalJSON() ([]byte, error) {
	rate := c.ComputeRate()
	return ([]byte(
		fmt.Sprintf(`{"current": %d, "rate": %f}`, c.Get(), rate))), nil
}
