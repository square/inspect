//Copyright (c) 2014 Square, Inc

package metricchecks

import (
	"github.com/square/inspect/metrics"
)

type Checker interface {
	// User must call NewScopeAndPackage before
	// inserting metric values
	NewScopeAndPackage() error

	// Metric values mey be inserted from a json package or the
	// metric context (if available)
	// User must insert metric values before running metric check
	InsertMetricValuesFromJSON() error
	// User must insert metric values before running metric check
	InsertMetricValuesFromContext(m *metrics.MetricContext) error

	// Runs metric check
	CheckAll() ([]CheckResult, error)
}

// Results are returned in the following format
type CheckResult struct {
	Name    string  // Name of check
	Message string  // Message associated with check
	Tags    string  // Who/what should be alerted
	Value   float64 // Value of the main metric being checked
}
