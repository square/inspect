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

/* StatsTimer

A StatTimer can be used to compute statistics for a timed operation
Arguments:
  timeUnit time.Duration - time unit to report statistics on
  nsamples int - number of samples to keep in-memory for stats computation

Example use:
  m := metrics.NewMetricContext("webapp")
  s := m.NewStatsTimer("latency", time.Millisecond, 100)

  func (wa *WebApp)  HandleQuery(w http.ResponseWriter, r *http.Request) {
	  stopWatch := s.Start()
	  .....
	  s.Stop(stopWatch)
  }

  pctile_95th, err := s.Percentile(95)

  if err == nil {
  	fmt.Printf("95th percentile latency: ", pctile_95th)
  }

*/

type StatsTimer struct {
	history  []int64
	idx      int
	mu       sync.RWMutex
	timeUnit time.Duration
}

const NOT_INITIALIZED = -1

// default percentiles to compute when serializing statstimer type
// to stdout/json
var PERCENTILES = []float64{50, 75, 95, 99, 99.9, 99.99, 99.999}

func NewStatsTimer(timeUnit time.Duration, nsamples int) *StatsTimer {

	s := new(StatsTimer)
	s.timeUnit = timeUnit
	s.history = make([]int64, nsamples)

	s.Reset()

	return s
}

func (s *StatsTimer) Reset() {
	for i := range s.history {
		s.history[i] = NOT_INITIALIZED
	}
}

func (s *StatsTimer) Start() *Timer {
	t := NewTimer()
	t.Start()
	return t
}

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

type Int64Slice []int64

func (a Int64Slice) Len() int           { return len(a) }
func (a Int64Slice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Int64Slice) Less(i, j int) bool { return a[i] < a[j] }

func (s *StatsTimer) Percentile(percentile float64) (float64, error) {
	// Nearest rank implementation
	// http://en.wikipedia.org/wiki/Percentile
	histLen := len(s.history)

	if percentile > 100 {
		return math.NaN(), errors.New("Invalid argument")
	}

	in := make([]int64, 0, histLen)
	for i := range s.history {
		if s.history[i] != NOT_INITIALIZED {
			in = append(in, s.history[i])
		}
	}

	filtLen := len(in)

	if filtLen < 1 {
		return math.NaN(), errors.New("No values")
	}

	// Since slices are zero-indexed, we are naturally rounded up
	nearest_rank := int((percentile / 100) * float64(filtLen))

	if nearest_rank == filtLen {
		nearest_rank = filtLen - 1
	}

	sort.Sort(Int64Slice(in))
	ret := float64(in[nearest_rank]) / float64(s.timeUnit.Nanoseconds())

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
	for _, p := range PERCENTILES {
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
