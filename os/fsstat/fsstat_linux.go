// Copyright (c) 2014 Square, Inc

// Package fsstat implements metrics collection related to filesystem usage
package fsstat

import (
	"bufio"
	"math"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/misc"
)

// FSStat represents file system statistics for all filesystems found
// on this OS
type FSStat struct {
	FS map[string]*PerFSStat
	m  *metrics.MetricContext
}

// New registers with metriccontext and collects filesystem stats every
// Step
func New(m *metrics.MetricContext, Step time.Duration) *FSStat {
	s := new(FSStat)
	s.FS = make(map[string]*PerFSStat, 0)
	s.m = m
	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			s.Collect()
		}
	}()
	return s
}

// Collect is run every step seconds to parse /etc/mstab
// and gather inode/disk usage metrics
func (s *FSStat) Collect() {
	file, err := os.Open("/etc/mtab")
	defer file.Close()
	if err != nil {
		return
	}
	// mark all objects as non-mounted to weed out
	// the ones that disappeared from last time we ran
	for _, o := range s.FS {
		o.IsMounted = false
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := strings.Split(scanner.Text(), " ")
		// ignore few types of mounts
		// man fstab
		switch f[2] {
		case "proc", "sysfs", "devpts", "none", "sunrpc", "swap", "bind", "ignore", "tmpfs", "binfmt_misc", "rpc_pipefs":
			continue
		}
		if strings.Contains(f[2], "fuse") {
			continue
		}
		o, ok := s.FS[f[1]]
		if !ok {
			o = NewPerFSStat(s.m, f[1])
			s.FS[f[1]] = o
		}
		o.IsMounted = true
		o.Collect()
	}
	// remove entries for mounts that no longer exist
	for name, o := range s.FS {
		if !o.IsMounted {
			o.Unregister()
			delete(s.FS, name)
		}
	}
}

// Return list of file systems sorted by Usage
type byUsage []*PerFSStat

func (a byUsage) Len() int           { return len(a) }
func (a byUsage) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byUsage) Less(i, j int) bool { return a[i].Usage() > a[j].Usage() }

// ByUsage returns an slice of *PerDiskStat entries sorted
// by usage
func (s *FSStat) ByUsage() []*PerFSStat {
	var v []*PerFSStat
	for _, o := range s.FS {
		if !math.IsNaN(o.Usage()) {
			v = append(v, o)
		}
	}
	sort.Sort(byUsage(v))
	return v
}

// PerFSStat represents type for filesystem specific information
// including associated metrics
type PerFSStat struct {
	m         *metrics.MetricContext
	mp        string
	IsMounted bool
	Name      string
	Bsize     *metrics.Gauge
	Blocks    *metrics.Gauge
	Bfree     *metrics.Gauge
	Bavail    *metrics.Gauge
	Files     *metrics.Gauge
	Ffree     *metrics.Gauge
	// Computed stats
	UsagePct     *metrics.Gauge
	FileUsagePct *metrics.Gauge
}

// NewPerFSStat registers with metriccontext for the particular filesystem
func NewPerFSStat(m *metrics.MetricContext, mp string) *PerFSStat {
	fs := new(PerFSStat)
	fs.mp = mp
	fs.Name = mp
	misc.InitializeMetrics(fs, m, "fsstat."+mp, true)
	return fs
}

// Unregister removes metrics from metric-context
func (s *PerFSStat) Unregister() {
	misc.UnregisterMetrics(s, s.m, "fsstat."+s.mp)
}

// Collect calls statfs and populates stats for a particular filesystem
func (s *PerFSStat) Collect() {
	// call statfs and populate metrics
	buf := new(syscall.Statfs_t)
	err := syscall.Statfs(s.mp, buf)
	if err != nil {
		return
	}

	s.Bsize.Set(float64(buf.Bsize))
	s.Blocks.Set(float64(buf.Blocks))
	s.Bfree.Set(float64(buf.Bfree))
	s.Bavail.Set(float64(buf.Bavail))
	s.Files.Set(float64(buf.Files))
	s.Ffree.Set(float64(buf.Ffree))
	s.UsagePct.Set(s.Usage())
	s.FileUsagePct.Set(s.FileUsage())
}

// Usage returns filesystem block usage in percentage
func (s *PerFSStat) Usage() float64 {
	total := s.Blocks.Get()
	free := s.Bfree.Get()
	return ((total - free) / total) * 100
}

// FileUsage returns filesystem files (inodes) usage in percentage
func (s *PerFSStat) FileUsage() float64 {
	total := s.Files.Get()
	free := s.Ffree.Get()
	return ((total - free) / total) * 100
}
