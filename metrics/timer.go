// Copyright (c) 2014 Square, Inc

package metrics

import (
	"time"
)

// Timer represents a type to store data for implementing
// a timer
type Timer struct {
	v          int64
	startValue int64
}

// NewTimer returns a Timer
func NewTimer() *Timer {
	t := new(Timer)
	return t
}

// Start initializes and starts a Timer
func (t *Timer) Start() {
	t.startValue = time.Now().UnixNano()
}

// Stop stops the Timer
func (t *Timer) Stop() int64 {
	t.v = time.Now().UnixNano() - t.startValue
	// TODO: need to verify if go uses monotonic clocks
	// if available
	if t.v < 0 {
		t.v = 0
	}
	return t.v
}

// Get returns the current value of timer or zero if the timer
// is still running
func (t *Timer) Get() int64 {
	return t.v
}
