// Copyright (c) 2014 Square, Inc

package misc

import (
	"math"
	"testing"
)

var parseUintTests = []struct {
	in       string
	expected uint64
}{
	{"10", 10},
	{"-10", 0},
	{"10e4", 0},
	{"abcd", 0},
	{"111111111111111111111111111111111111111111111111111111111111111111111111111111", 0},
}

func TestParseUint(t *testing.T) {
	for _, tt := range parseUintTests {
		actual := ParseUint(tt.in)
		if actual != tt.expected {
			t.Errorf("ParseUint(%v) => %v, want %v", tt.in, actual, tt.expected)
		}
	}
}

var parseFloatTests = []struct {
	in       string
	expected float64
}{
	{"10.0", 10.0},
	{"abcde", math.NaN()},
}

func TestParseFloat(t *testing.T) {
	for _, tt := range parseFloatTests {
		actual := ParseFloat(tt.in)
		if actual != tt.expected && !math.IsNaN(tt.expected) {
			t.Errorf("ParseFloat(%v) => %v, want %v", tt.in, actual, tt.expected)
		}
	}
}
