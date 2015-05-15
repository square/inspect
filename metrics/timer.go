// Copyright (c) 2014 Square, Inc

package metrics

import (
	"time"
)

type Timer struct {
	v       int64
	start_v int64
}

// Timer
func NewTimer() *Timer {
	t := new(Timer)
	return t
}

func (t *Timer) Start() {
	t.start_v = time.Now().UnixNano()
}

func (t *Timer) Stop() int64 {
	t.v = time.Now().UnixNano() - t.start_v
	// need to verify if go uses monotonic clocks
	// if available
	if t.v < 0 {
		t.v = 0
	}
	return t.v
}

func (t *Timer) Get() int64 {
	return t.v
}
