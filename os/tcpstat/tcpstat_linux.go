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

var root = "/" // to make testing easy

// TCPStat represents statistics about various tcp indicators
// and is automatically initialized.
// Caution: reflection is used to read this struct to discover names
type TCPStat struct {
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
	Extended     *ExtendedMetrics
	// not exported
	m *metrics.MetricContext
}

// ExtendedMetrics represents extended statistics about various tcp indicators
// and is automatically initialized.
// Caution: reflection is used to read this struct to discover names
type ExtendedMetrics struct {
	SyncookiesSent   *metrics.Counter
	SyncookiesRecv   *metrics.Counter
	SyncookiesFailed *metrics.Counter
	ListenOverflows  *metrics.Counter
	ListenDrops      *metrics.Counter
}

// New starts metrics collection every Step and registers with
// metricscontext
func New(m *metrics.MetricContext, Step time.Duration) *TCPStat {
	s := new(TCPStat)
	s.m = m
	s.Extended = new(ExtendedMetrics)
	// initialize all metrics and register them
	misc.InitializeMetrics(s, m, "tcpstat", true)
	misc.InitializeMetrics(s.Extended, m, "tcpstat.ext", true)
	// collect once
	s.Collect()
	// collect metrics every Step
	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			s.Collect()
		}
	}()
	return s
}

// Collect populates TCPStat by reading /proc/net/snmp and /proc/net/netstat
func (s *TCPStat) Collect() {
	populateMetrics(s.m, s, root+"proc/net/snmp", "Tcp:")
	populateMetrics(s.m, s.Extended, root+"proc/net/netstat", "TcpExt:")
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
