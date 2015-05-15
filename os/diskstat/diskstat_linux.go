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

type DiskStat struct {
	Disks   map[string]*PerDiskStat
	m       *metrics.MetricContext
	blkdevs map[string]bool
}

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
type ByUsage []*PerDiskStat

func (a ByUsage) Len() int           { return len(a) }
func (a ByUsage) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByUsage) Less(i, j int) bool { return a[i].Usage() > a[j].Usage() }

// ByUsage() returns an slice of *PerDiskStat entries sorted
// by usage
func (c *DiskStat) ByUsage() []*PerDiskStat {
	v := make([]*PerDiskStat, 0)
	for _, o := range c.Disks {
		if !math.IsNaN(o.Usage()) {
			v = append(v, o)
		}
	}
	sort.Sort(ByUsage(v))
	return v
}

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
		d := o.Metrics
		d.ReadCompleted.Set(f[0])
		d.ReadMerged.Set(f[1])
		d.ReadSectors.Set(f[2])
		d.ReadSpentMsecs.Set(f[3])
		d.WriteCompleted.Set(f[4])
		d.WriteMerged.Set(f[5])
		d.WriteSectors.Set(f[6])
		d.WriteSpentMsecs.Set(f[7])
		d.IOInProgress.Set(float64(f[8]))
		d.IOSpentMsecs.Set(f[9])
		d.WeightedIOSpentMsecs.Set(f[10])
		d.SectorSize.Set(float64(sectorSize))
	}
}

type PerDiskStat struct {
	Metrics *PerDiskStatMetrics
	m       *metrics.MetricContext
	Name    string
}

type PerDiskStatMetrics struct {
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
}

func NewPerDiskStat(m *metrics.MetricContext, blkdev string) *PerDiskStat {
	c := new(PerDiskStat)
	c.Name = blkdev
	c.Metrics = new(PerDiskStatMetrics)
	// initialize all metrics and register them
	misc.InitializeMetrics(c.Metrics, m, "diskstat."+blkdev, true)
	return c
}

// Usage returns approximate measure of disk usage
// (time spent doing IO / wall clock time)
func (s *PerDiskStat) Usage() float64 {
	o := s.Metrics
	return ((o.IOSpentMsecs.ComputeRate()) / 1000) * 100
}
