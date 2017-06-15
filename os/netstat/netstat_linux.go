// Copyright (c) 2015 Square, Inc

// Package netstat implements metrics collection related to TCP and UDP
package netstat

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/misc"
)

var root = "/" // to make testing easy

// NetStat is a holder for TCP and UDP statistics.
type NetStat struct {
	TCPStat         TCPStat
	UDPStat         UDPStat
	ExtendedMetrics ExtendedMetrics

	m *metrics.MetricContext
}

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
}

// UDPStat represents statistics about UDP. Reflection is used to map field names to
// kernel data.
type UDPStat struct {
	InDatagrams  *metrics.Counter
	NoPorts      *metrics.Counter
	InErrors     *metrics.Counter
	OutDatagrams *metrics.Counter
	RcvbufErrors *metrics.Counter
	SndbufErrors *metrics.Counter
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
func New(m *metrics.MetricContext, Step time.Duration) *NetStat {
	s := &NetStat{m: m}
	// initialize all metrics and register them
	misc.InitializeMetrics(&s.TCPStat, m, "tcpstat", true)
	misc.InitializeMetrics(&s.UDPStat, m, "udpstat", true)
	misc.InitializeMetrics(&s.ExtendedMetrics, m, "tcpstat.ext", true)
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

// Collect populates NetStat by reading /proc/net/snmp and /proc/net/netstat
func (s *NetStat) Collect() {
	if snmp, err := ioutil.ReadFile(root + "proc/net/snmp"); err == nil {
		populateMetrics(s.m, &s.TCPStat, snmp, "Tcp:")
		populateMetrics(s.m, &s.UDPStat, snmp, "Udp:")
	}
	if netstat, err := ioutil.ReadFile(root + "proc/net/netstat"); err == nil {
		populateMetrics(s.m, &s.ExtendedMetrics, netstat, "TcpExt:")
	}
}

// Unexported functions
func populateMetrics(m *metrics.MetricContext, s interface{}, data []byte, prefix string) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
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
