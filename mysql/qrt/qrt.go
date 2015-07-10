package qrt

import (
	"sort"
)

type MysqlQrtBucket struct {
	time  string
	count int64
	total float64
}

type MysqlQrtHistogram []MysqlQrtBucket

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
	var p_ix float64
	var cur_pctl int64
	var total int64
	var pctl float64

	// Find the total number of entries in histogram
	total = h.Count()

	// Multiply the total number of values in the data set by the percentile, which will give you the index.
	p_ix = (float64(total) * float64(p)) * .01

	// Order all of the values in the data set in ascending order (least to greatest).
	sort.Sort(MysqlQrtHistogram(h))

	// Find the tgt percentile
	for i, v := range h {
		cur_pctl += v.count
		if float64(cur_pctl) >= p_ix {
			pctl = h[i].total / float64(h[i].count) * 1000 // Convert to ms
			break
		}
	}

	return pctl
}
