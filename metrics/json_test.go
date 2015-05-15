// Copyright (c) 2014 Square, Inc

package metrics

import "testing"
import "strings"
import "net/http"
import "net/http/httptest"

func TestJsonHandler(t *testing.T) {
	m := NewMetricContext("test")
	g1 := NewGauge() //g1 should be NaN
	m.Register(g1, "testGauge1")
	g2 := NewGauge()
	m.Register(g2, "testGauge2")
	g2.Set(float64(42)) // g2 is not NaN
	req, err := http.NewRequest("GET", "metrics.json", nil)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	m.HttpJsonHandler(response, req)
	if strings.Contains(response.Body.String(), "NaN") {
		t.Errorf("Did not expect a NaN value in response, but got: " +
			response.Body.String())
	}
}
