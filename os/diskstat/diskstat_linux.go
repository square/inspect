// Copyright (c) 2014 Square, Inc

// Package diskstat implements metrics collection related to disk IO usage
package diskstat

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"sort"
	"time"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/misc"
)

// DiskStat represents statistics collected for all disks (block devices) present
// on the current operating system.
type DiskStat struct {
	Disks   map[string]*PerDiskStat
	m       *metrics.MetricContext
	blkdevs map[string]bool
}

// New registers statistics with metrics context and starts collection of metrics
// every Step seconds
func New(m *metrics.MetricContext, Step time.Duration) *DiskStat {
	s := new(DiskStat)
	s.Disks = make(map[string]*PerDiskStat, 6)
	s.m = m
	s.RefreshBlkDevList() // perhaps call this once in a while
	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			s.Collect()
		}
	}()
	return s
}

// Return list of disks sorted by Usage
type byUsage []*PerDiskStat

func (a byUsage) Len() int           { return len(a) }
func (a byUsage) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byUsage) Less(i, j int) bool { return a[i].Usage() > a[j].Usage() }

// ByUsage returns an slice of *PerDiskStat entries sorted
// by usage
func (s *DiskStat) ByUsage() []*PerDiskStat {
	var v []*PerDiskStat
	for _, o := range s.Disks {
		if !math.IsNaN(o.Usage()) {
			v = append(v, o)
		}
	}
	sort.Sort(byUsage(v))
	return v
}

// RefreshBlkDevList walks through /sys/block and updates list of
// block devices.
func (s *DiskStat) RefreshBlkDevList() {
	var blkdevs = make(map[string]bool)
	// block devices
	o, err := ioutil.ReadDir("/sys/block")
	if err == nil {
		for _, d := range o {
			blkdevs[path.Base(d.Name())] = true
		}
	}
	s.blkdevs = blkdevs
}

// Collect walks through /proc/diskstats and updates relevant metrics
func (s *DiskStat) Collect() {
	file, err := os.Open("/proc/diskstats")
	defer file.Close()
	if err != nil {
		return
	}
	var blkdev string
	var major, minor uint64
	var f [11]uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fmt.Sscanf(scanner.Text(),
			"%d %d %s %d %d %d %d %d %d %d %d %d %d %d",
			&major, &minor, &blkdev, &f[0], &f[1], &f[2], &f[3],
			&f[4], &f[5], &f[6], &f[7], &f[8], &f[9], &f[10])
		// skip loop/ram/dm drives
		if major == 1 || major == 7 || major == 253 {
			continue
		}
		// skip collecting for individual partitions
		_, ok := s.blkdevs[blkdev]
		if !ok {
			continue
		}
		o, ok := s.Disks[blkdev]
		if !ok {
			o = NewPerDiskStat(s.m, blkdev)
			s.Disks[blkdev] = o
		}
		sectorSize := misc.ReadUintFromFile("/sys/block/" + blkdev + "/queue/hw_sector_size")
		o.ReadCompleted.Set(f[0])
		o.ReadMerged.Set(f[1])
		o.ReadSectors.Set(f[2])
		o.ReadSpentMsecs.Set(f[3])
		o.WriteCompleted.Set(f[4])
		o.WriteMerged.Set(f[5])
		o.WriteSectors.Set(f[6])
		o.WriteSpentMsecs.Set(f[7])
		o.IOInProgress.Set(float64(f[8]))
		o.IOSpentMsecs.Set(f[9])
		o.WeightedIOSpentMsecs.Set(f[10])
		o.SectorSize.Set(float64(sectorSize))
	}
}

// PerDiskStat represents disk statistics for a particular disk
type PerDiskStat struct {
	ReadCompleted        *metrics.Counter
	ReadMerged           *metrics.Counter
	ReadSectors          *metrics.Counter
	ReadSpentMsecs       *metrics.Counter
	WriteCompleted       *metrics.Counter
	WriteMerged          *metrics.Counter
	WriteSectors         *metrics.Counter
	WriteSpentMsecs      *metrics.Counter
	IOInProgress         *metrics.Gauge
	IOSpentMsecs         *metrics.Counter
	WeightedIOSpentMsecs *metrics.Counter
	SectorSize           *metrics.Gauge
	m                    *metrics.MetricContext
	Name                 string
}

// NewPerDiskStat registers with metriccontext for a particular disk (block device)
func NewPerDiskStat(m *metrics.MetricContext, blkdev string) *PerDiskStat {
	s := new(PerDiskStat)
	s.Name = blkdev
	// initialize all metrics and register them
	misc.InitializeMetrics(s, m, "diskstat."+blkdev, true)
	return s
}

// Usage returns approximate measure of disk usage
// (time spent doing IO / wall clock time)
func (s *PerDiskStat) Usage() float64 {
	return ((s.IOSpentMsecs.ComputeRate()) / 1000) * 100
}
