// Copyright (c) 2014 Square, Inc

package metrics

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"
)

// XXX: evaluate merging the types with individual definitions
// XXX: MetricContext holds registry of names and associated
// metrics

// MetricJSON is a type for serializing any metric type
type MetricJSON struct {
	Type  string
	Name  string
	Value interface{}
}

// EncodeJSON is a streaming encoder that writes all metrics passing filter
// to writer w as JSON
func (m *MetricContext) EncodeJSON(w io.Writer) error {
	w.Write([]byte("["))
	// JSON disallows trailing-comma
	prependComma := false
	for name, c := range m.Counters {
		m.writeJSON(w, name, c, &prependComma)
	}

	for name, c := range m.BasicCounters {
		m.writeJSON(w, name, c, &prependComma)
	}
	for name, g := range m.Gauges {
		m.writeJSON(w, name, g, &prependComma)
	}

	for name, s := range m.StatsTimers {
		m.writeJSON(w, name, s, &prependComma)
	}
	w.Write([]byte("]"))
	return nil
}

// unexported functions
func (m *MetricContext) writeJSON(w io.Writer, name string, v interface{}, prependComma *bool) {
	b, err := m.marshalMetricJSON(name, v)
	if err == nil {
		if *prependComma {
			w.Write([]byte(","))
		}
		w.Write(b)
		*prependComma = true
	}
}

func (m *MetricContext) marshalMetricJSON(name string, v interface{}) ([]byte, error) {
	o := new(MetricJSON)
	if !m.OutputFilter(name, v) {
		return nil, errors.New("filtered")
	}
	o.Type = reflect.TypeOf(v).String()
	o.Name = name
	o.Value = v
	return json.Marshal(o)
}
