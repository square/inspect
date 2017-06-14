// Copyright (c) 2014 Square, Inc

package metrics

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// package initialization code
// sets up a ticker to "cache" time

const jiffy = 100

var ticks int64
var ticker = time.NewTicker(time.Millisecond * jiffy)

func init() {
	start := time.Now().UnixNano()
	go func() {
		for t := range ticker.C {
			atomic.StoreInt64(&ticks, t.UnixNano()-start)
		}
	}()
}

// Counter represents an always incrementing metric type
// Counter differs from BasicCounter by having additional
// fields for computing rate. Operations on counter hold
// a mutex. use BasicCounter if you need lock-free counters
type Counter struct {
	v             uint64
	p             uint64
	rate          float64
	ticksPrevious int64
	ticksCurrent  int64
	mu            sync.Mutex
}

// NewCounter initializes and returns a new counter
func NewCounter() *Counter {
	c := new(Counter)
	c.Reset()
	return c
}

// Reset - resets all internal variables to defaults
// Usually called from NewCounter but useful if you have to
// re-use and existing object
func (c *Counter) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.rate = 0.0
	c.ticksPrevious = 0
	c.ticksCurrent = 0
	c.v = 0
	c.p = 0
}

// Set - Sets counter to input value. This is useful if you are reading a metric
// that is already a counter
func (c *Counter) Set(v uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ticksCurrent = atomic.LoadInt64(&ticks)
	c.v = v

	// initialize previous values to current if counter
	// overflows or if this is our first value
	if c.ticksPrevious == 0 || c.p > c.v {
		c.p = c.v
		c.ticksPrevious = c.ticksCurrent
	}
}

// Add - add input value to counter
func (c *Counter) Add(delta uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ticksCurrent = atomic.LoadInt64(&ticks)
	c.v += delta

	// initialize previous values to current if counter
	// overflows or if this is our first value
	if c.ticksPrevious == 0 || c.p > c.v {
		c.p = c.v
		c.ticksPrevious = c.ticksCurrent
	}
}

// Get - returns current value of counter
func (c *Counter) Get() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.v
}

// ComputeRate calculates the rate of change of counter per
// second. (acquires a lock)
func (c *Counter) ComputeRate() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	rate := 0.0

	deltaTime := c.ticksCurrent - c.ticksPrevious
	deltaValue := c.v - c.p

	// we have two samples, compute rate and
	// cache it away
	if deltaTime > 0 && c.v >= c.p {
		rate = (float64(deltaValue) / float64(deltaTime)) * NsInSec
		// update baseline
		c.p = c.v
		c.ticksPrevious = c.ticksCurrent
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
