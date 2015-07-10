package qrt

import (
	"sort"
)

// MysqlQrtBucket : https://www.percona.com/doc/percona-server/5.6/diagnostics/response_time_distribution.html
// Represents a row from information_schema.Query_Reponse_Time
type MysqlQrtBucket struct {
	time  string
	count int64
	total float64
}

// MysqlQrtHistogram represents a histogram containing MySQLQRTBuckets. Where each bucket is a bin.
type MysqlQrtHistogram []MysqlQrtBucket

// NewMysqlQrtBucket Public way to return a QRT bucket to be appended to a Histogram
func NewMysqlQrtBucket(time string, count int64, total float64) MysqlQrtBucket {
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
func (h MysqlQrtHistogram) Percentile(p float32) float64 {
	var pIx float64
	var curPctl int64
	var total int64
	var pctl float64

	// Find the total number of entries in histogram
	total = h.Count()

	// Multiply the total number of values in the data set by the percentile, which will give you the index.
	pIx = float64(total) * (float64(p) * .01)

	// Order all of the values in the data set in ascending order (least to greatest).
	sort.Sort(MysqlQrtHistogram(h))

	// Find the tgt percentile, make it an average because using buckets of all same value entries
	for i, v := range h {
		curPctl += v.count
		if float64(curPctl) >= pIx {
			pctl = h[i].total / float64(h[i].count)
			break
		}
	}

	return pctl
}
