package qrt

import (
	"testing"
)

func TestPercentile(t *testing.T) {
	p := [13]float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.75, 0.8, 0.9, 0.95, 0.99, 0.999}
	expectedresults := [13]float64{10, 20, 20, 20, 20, 30, 30, 30, 40, 40, 40, 40, 40}

	h := MysqlQrtHistogram{
		{0, 0, 0},
		{10, 2, 20},
		{20, 5, 100},
		{30, 4, 120},
		{40, 3, 120},
	}

	for i, x := range p {
		result := h.Percentile(x)
		if result != expectedresults[i] {
			t.Errorf("For Percentile: %v\tExpected: %v\tGot: %v\n", x, expectedresults[i], result)
		}
	}
}
