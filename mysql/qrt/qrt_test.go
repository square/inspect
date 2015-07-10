package qrt

import (
	"testing"
)

func TestPercentile(t *testing.T) {
	var h MysqlQrtHistogram

	h = append(h, NewMysqlQrtBucket(string(85), 1, 85))
	h = append(h, NewMysqlQrtBucket(string(34), 1, 34))
	h = append(h, NewMysqlQrtBucket(string(42), 1, 42))
	h = append(h, NewMysqlQrtBucket(string(51), 1, 51))
	h = append(h, NewMysqlQrtBucket(string(84), 1, 84))
	h = append(h, NewMysqlQrtBucket(string(86), 1, 86))
	h = append(h, NewMysqlQrtBucket(string(78), 1, 78))
	h = append(h, NewMysqlQrtBucket(string(85), 1, 85))
	h = append(h, NewMysqlQrtBucket(string(87), 1, 87))
	h = append(h, NewMysqlQrtBucket(string(69), 1, 69))
	h = append(h, NewMysqlQrtBucket(string(94), 1, 94))
	h = append(h, NewMysqlQrtBucket(string(74), 1, 74))
	h = append(h, NewMysqlQrtBucket(string(65), 1, 65))
	h = append(h, NewMysqlQrtBucket(string(56), 1, 56))
	h = append(h, NewMysqlQrtBucket(string(97), 1, 97))
	if h.Percentile(90) != 94 {
		t.FailNow()
	}

}
