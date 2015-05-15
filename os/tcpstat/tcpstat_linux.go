// Copyright (c) 2015 Square, Inc

// Package tcpstat implements metrics collection related to TCP
package tcpstat

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/misc"
)

type TCPStat struct {
	Metrics *TCPStatMetrics
	m       *metrics.MetricContext
}

func New(m *metrics.MetricContext, Step time.Duration) *TCPStat {
	s := new(TCPStat)
	s.Metrics = NewTCPStatMetrics(m, Step)
	return s
}

// Caution: reflection is used to read this struct to discover names
// Do not add new types
type TCPStatMetrics struct {
	MaxConn      *metrics.Gauge
	ActiveOpens  *metrics.Counter
	PassiveOpens *metrics.Counter
	AttemptFails *metrics.Counter
	EstabResets  *metrics.Counter
	CurrEstab    *metrics.Gauge
	InSegs       *metrics.Counter
	OutSegs      *metrics.Counter
	RetransSegs  *metrics.Counter
	InErrs       *metrics.Counter
	OutRsts      *metrics.Counter
	Extended     *TCPStatExtendedMetrics
	//
	m *metrics.MetricContext
}

// Caution: reflection is used to read this struct to discover names
// Do not add new types
type TCPStatExtendedMetrics struct {
	SyncookiesSent   *metrics.Counter
	SyncookiesRecv   *metrics.Counter
	SyncookiesFailed *metrics.Counter
	ListenOverflows  *metrics.Counter
	ListenDrops      *metrics.Counter
}

func NewTCPStatMetrics(m *metrics.MetricContext, Step time.Duration) *TCPStatMetrics {
	c := new(TCPStatMetrics)
	c.m = m
	c.Extended = new(TCPStatExtendedMetrics)
	// initialize all metrics and register them
	misc.InitializeMetrics(c, m, "tcpstat", true)
	misc.InitializeMetrics(c.Extended, m, "tcpstat.ext", true)
	// collect once
	c.Collect()
	// collect metrics every Step
	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			c.Collect()
		}
	}()
	return c
}

func (s *TCPStatMetrics) Collect() {
	populateMetrics(s.m, s, "/proc/net/snmp", "Tcp:")
	populateMetrics(s.m, s.Extended, "/proc/net/netstat", "TcpExt:")
}

// Unexported functions
func populateMetrics(m *metrics.MetricContext, s interface{}, filename string, prefix string) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(file)
	var keys []string
	var values []string
	seen := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, prefix) && !seen {
			seen = true
			keys = regexp.MustCompile("[:\\s]+").Split(scanner.Text(), -1)
		}
		if strings.HasPrefix(line, prefix) && seen {
			values = regexp.MustCompile("[:\\s]+").Split(scanner.Text(), -1)
		}
	}
	misc.SetMetrics(m, s, keys, values)
}
