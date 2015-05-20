// Copyright (c) 2014 Square, Inc

package metrics

import (
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// package initialization code
// sets up a ticker to "cache" time

var ticks int64

func init() {
	start := time.Now().UnixNano()
	ticker := time.NewTicker(time.Millisecond * jiffy)
	go func() {
		for t := range ticker.C {
			atomic.StoreInt64(&ticks, t.UnixNano()-start)
		}
	}()
}

// OutputFilterFunc represents a function that is used to filter
// metrics from being reported out from JSON handler
type OutputFilterFunc func(name string, v interface{}) bool

// MetricContext represents a reference to all metrics registered
// by name within a namespace
type MetricContext struct {
	namespace     string
	Counters      map[string]*Counter
	Gauges        map[string]*Gauge
	BasicCounters map[string]*BasicCounter
	StatsTimers   map[string]*StatsTimer
	OutputFilter  OutputFilterFunc
}

// Creates a new metric context. A metric context specifies a namespace
// time duration that is used as step and number of samples to keep
// in-memory
// Arguments:
// namespace - namespace that all metrics in this context belong to

const jiffy = 100

//nanoseconds in a second represented in float64
const NsInSec = float64(time.Second)

// NewMetricContext initializes a MetricContext with the input namespace
// and returns it
func NewMetricContext(namespace string) *MetricContext {
	m := new(MetricContext)
	m.namespace = namespace
	m.Counters = make(map[string]*Counter, 0)
	m.Gauges = make(map[string]*Gauge, 0)
	m.BasicCounters = make(map[string]*BasicCounter, 0)
	m.StatsTimers = make(map[string]*StatsTimer, 0)
	m.OutputFilter = func(name string, v interface{}) bool {
		return true
	}

	return m
}

// Register registers a metric with metriccontext
func (m *MetricContext) Register(v interface{}, name string) {
	switch v := v.(type) {
	case *BasicCounter:
		m.BasicCounters[name] = v
	case *Counter:
		m.Counters[name] = v
	case *Gauge:
		m.Gauges[name] = v
	case *StatsTimer:
		m.StatsTimers[name] = v
	}
}

// Unregister unregisters a metric with metriccontext
func (m *MetricContext) Unregister(v interface{}, name string) {
	switch v.(type) {
	case *BasicCounter:
		delete(m.BasicCounters, name)
	case *Counter:
		delete(m.Counters, name)
	case *Gauge:
		delete(m.Gauges, name)
	case *StatsTimer:
		delete(m.StatsTimers, name)
	}
}

// HttpJsonHandler setups a handler for exposing metrics via JSON over HTTP
func (m *MetricContext) HttpJsonHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	m.EncodeJSON(w)
	w.Write([]byte("\n")) // Be nice to curl
}

// unexported functions
func parseURL(url string) []string {
	path := strings.SplitN(url, "metrics.json", 2)[1]
	levels := strings.Split(path, "/")
	return levels[1:]
}
