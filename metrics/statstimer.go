// Copyright (c) 2014 Square, Inc

package metrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// StatsTimer represents a metric of type statstimer
type StatsTimer struct {
	history  []int64
	idx      int
	mu       sync.RWMutex
	timeUnit time.Duration
}

const notInitialized = -1

// Percentiles are default percentiles to compute when serializing statstimer type
// to stdout/json
var Percentiles = []float64{50, 75, 95, 99, 99.9, 99.99, 99.999}

// NewStatsTimer initializes and returns a StatsTimer.
// StatTimer can be used to compute statistics for a timed operation.
// Arguments:
//  timeUnit time.Duration - time unit to report statistics on
//  nsamples int - number of samples to keep in-memory for stats computation
// Example:
//  m := metrics.NewMetricContext("webapp")
//  s := m.NewStatsTimer("latency", time.Millisecond, 100)
//  func (wa *WebApp)  HandleQuery(w http.ResponseWriter, r *http.Request) {
//     stopWatch := s.Start()
//     ... do work...
//     s.Stop(stopWatch)
//  }
//  pctile95, err := s.Percentile(95)
//  if err == nil {
//    fmt.Printf("95th percentile for latency: ", pctile95)
//  }
func NewStatsTimer(timeUnit time.Duration, nsamples int) *StatsTimer {
	s := new(StatsTimer)
	s.timeUnit = timeUnit
	s.history = make([]int64, nsamples)

	s.Reset()

	return s
}

// Reset - resets the stat of StatsTimer
func (s *StatsTimer) Reset() {
	for i := range s.history {
		s.history[i] = notInitialized
	}
}

// Start - Start a stopWatch for the StatsTimer and returns it
func (s *StatsTimer) Start() *Timer {
	t := NewTimer()
	t.Start()
	return t
}

// Stop - Stops the stopWatch for the StatsTimer.
func (s *StatsTimer) Stop(t *Timer) float64 {
	delta := t.Stop()

	// Store current value in history
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history[s.idx] = delta
	s.idx++
	if s.idx == len(s.history) {
		s.idx = 0
	}
	return float64(delta) / float64(s.timeUnit.Nanoseconds())
}

// TODO: move stats implementation to a dedicated package
type int64Slice []int64

func (a int64Slice) Len() int           { return len(a) }
func (a int64Slice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a int64Slice) Less(i, j int) bool { return a[i] < a[j] }

// Percentile returns the value at input percentile
// Implementation is based on Nearest rank
// http://en.wikipedia.org/wiki/Percentile
func (s *StatsTimer) Percentile(percentile float64) (float64, error) {
	histLen := len(s.history)

	if percentile > 100 {
		return math.NaN(), errors.New("Invalid argument")
	}

	in := make([]int64, 0, histLen)
	for i := range s.history {
		if s.history[i] != notInitialized {
			in = append(in, s.history[i])
		}
	}

	filtLen := len(in)

	if filtLen < 1 {
		return math.NaN(), errors.New("No values")
	}

	// Since slices are zero-indexed, we are naturally rounded up
	nearestRank := int((percentile / 100) * float64(filtLen))

	if nearestRank == filtLen {
		nearestRank = filtLen - 1
	}

	sort.Sort(int64Slice(in))
	ret := float64(in[nearestRank]) / float64(s.timeUnit.Nanoseconds())

	return ret, nil
}

// MarshalJSON returns a byte slice containing representation of
// StatsTimer
func (s *StatsTimer) MarshalJSON() ([]byte, error) {
	type percentileData struct {
		percentile string
		value      float64
	}
	var pctiles []percentileData
	for _, p := range Percentiles {
		percentile, err := s.Percentile(p)
		stuff := fmt.Sprintf("%.6f", p)
		if err == nil {
			pctiles = append(pctiles, percentileData{stuff, percentile})
		}
	}
	data := struct {
		Percentiles []percentileData
	}{
		pctiles,
	}
	return json.Marshal(data)
}
