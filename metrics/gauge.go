// Copyright (c) 2014 Square, Inc

package metrics

import (
	"encoding/json"
	"math"
	"sync"
)

// Gauges
type Gauge struct {
	v  float64
	mu sync.RWMutex
}

// NewGauge initializes a Gauge and returns it
func NewGauge() *Gauge {
	g := new(Gauge)
	g.Reset()
	return g
}

// Reset() all values are reset to defaults
// Usually called from NewGauge but useful if you have to
// re-use and existing object
func (g *Gauge) Reset() {
	g.v = math.NaN()
}

// Set value of Gauge
func (g *Gauge) Set(v float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.v = v
}

// Get value of Gauge
func (g *Gauge) Get() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.v
}

// MarshalJSON returns a byte slice of JSON representation of
// Gauge
func (g *Gauge) MarshalJSON() ([]byte, error) {
	return json.Marshal(g.v)
}
