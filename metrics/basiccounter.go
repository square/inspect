// Copyright (c) 2014 Square, Inc

package metrics

import (
	"encoding/json"
	"sync/atomic"
)

// BasicCounter is a minimal counter(uint64) - all operations are atomic
// Usage:
//   b := metrics.NewBasicCounter()
//   b.Add(1)
//   b.Get()

func NewBasicCounter() *BasicCounter {
	c := new(BasicCounter)
	c.Reset()
	return c
}

type BasicCounter uint64

// Reset counter to zero
func (c *BasicCounter) Reset() {
	atomic.StoreUint64((*uint64)(c), 0)
}

// Set counter to value v.
func (c *BasicCounter) Set(v uint64) {
	atomic.StoreUint64((*uint64)(c), v)
}

// Add delta to counter value v
func (c *BasicCounter) Add(delta uint64) {
	atomic.AddUint64((*uint64)(c), delta)
}

// Get value of counter
func (c *BasicCounter) Get() uint64 {
	return uint64(*c)
}

// MarshalJSON returns a byte slice of JSON representation of
// basiccounter
func (c *BasicCounter) MarshalJSON() ([]byte, error) {
	return json.Marshal(uint64(*c))
}
