package qrt

import (
	"sort"
)

// MysqlQrtBucket : https://www.percona.com/doc/percona-server/5.6/diagnostics/response_time_distribution.html
// Represents a row from information_schema.Query_Response_Time
type MysqlQrtBucket struct {
	time  float64
	count int64
	total float64
}

// MysqlQrtHistogram represents a histogram containing MySQLQRTBuckets. Where each bucket is a bin.
type MysqlQrtHistogram []MysqlQrtBucket

// NewMysqlQrtBucket Public way to return a QRT bucket to be appended to a Histogram
func NewMysqlQrtBucket(time float64, count int64, total float64) MysqlQrtBucket {
	return MysqlQrtBucket{time, count, total}
}

// Sort for QRT Histogram
func (h MysqlQrtHistogram) Len() int      { return len(h) }
func (h MysqlQrtHistogram) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h MysqlQrtHistogram) Less(i, j int) bool {
	return h[i].time < h[j].time
}

// Count for QRT Histogram
func (h MysqlQrtHistogram) Count() int64 {
	var total int64
	total = 0

	for _, v := range h {
		total += v.count
	}

	return total
}

// Percentile for QRTHistogram
// p should be p/100 where p is requested percentile (example: 0.10 for 10th percentile)
// Percentile is defined as the weighted of the percentiles of
// lowest bin that is greater than the requested percentile rank
func (h MysqlQrtHistogram) Percentile(p float64) float64 {
	var r float64
	var curPctl int64
	var pctl float64

	// Order all of the values in the data set in ascending order (least to greatest).
	sort.Sort(MysqlQrtHistogram(h))

	// Rank = N * P
	r = float64(h.Count()) * p

	// Find the tgt percentile, make it an average because using histogram buckets
	for i, v := range h {
		curPctl += v.count
		if float64(curPctl) >= r {
			pctl = h[i].total / float64(h[i].count)
			break
		}
	}

	return pctl
}
