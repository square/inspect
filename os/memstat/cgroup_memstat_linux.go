// Copyright (c) 2014 Square, Inc

package memstat

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/misc"
)

type CgroupStat struct {
	Cgroups    map[string]*PerCgroupStat
	m          *metrics.MetricContext
	Mountpoint string
}

func NewCgroupStat(m *metrics.MetricContext, Step time.Duration) *CgroupStat {
	c := new(CgroupStat)
	c.m = m
	c.Cgroups = make(map[string]*PerCgroupStat, 1)

	mountpoint, err := misc.FindCgroupMount("memory")
	if err != nil {
		return c
	}
	c.Mountpoint = mountpoint

	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			c.Collect(mountpoint)
		}
	}()

	return c
}

func (c *CgroupStat) Collect(mountpoint string) {

	cgroups, err := misc.FindCgroups(mountpoint)
	if err != nil {
		return
	}

	// stop tracking cgroups which don't exist
	// anymore or have no tasks
	cgroupsMap := make(map[string]bool, len(cgroups))
	for _, cgroup := range cgroups {
		cgroupsMap[cgroup] = true
	}

	for cgroup, _ := range c.Cgroups {
		_, ok := cgroupsMap[cgroup]
		if !ok {
			perCgroupStat, ok := c.Cgroups[cgroup]
			if ok {
				perCgroupStat.Unregister()
			}
			delete(c.Cgroups, cgroup)
		}
	}

	for _, cgroup := range cgroups {
		_, ok := c.Cgroups[cgroup]
		if !ok {
			c.Cgroups[cgroup] = NewPerCgroupStat(c.m, cgroup, mountpoint)
		}
		c.Cgroups[cgroup].Collect()
	}

}

// Per Cgroup functions
type PerCgroupStat struct {
	m *metrics.MetricContext
	// memory.stat
	Cache                     *metrics.Gauge
	Rss                       *metrics.Gauge
	Mapped_file               *metrics.Gauge
	Pgpgin                    *metrics.Gauge
	Pgpgout                   *metrics.Gauge
	Swap                      *metrics.Gauge
	Active_anon               *metrics.Gauge
	Inactive_anon             *metrics.Gauge
	Active_file               *metrics.Gauge
	Inactive_file             *metrics.Gauge
	Unevictable               *metrics.Gauge
	Hierarchical_memory_limit *metrics.Gauge
	Hierarchical_memsw_limit  *metrics.Gauge
	Total_cache               *metrics.Gauge
	Total_rss                 *metrics.Gauge
	Total_mapped_file         *metrics.Gauge
	Total_pgpgin              *metrics.Gauge
	Total_pgpgout             *metrics.Gauge
	Total_swap                *metrics.Gauge
	Total_inactive_anon       *metrics.Gauge
	Total_active_anon         *metrics.Gauge
	Total_inactive_file       *metrics.Gauge
	Total_active_file         *metrics.Gauge
	Total_unevictable         *metrics.Gauge
	// memory.soft_limit_in_bytes
	Soft_Limit_In_Bytes *metrics.Gauge
	// Approximate usage in bytes
	UsageInBytes *metrics.Gauge
	path         string
	prefix       string
}

func NewPerCgroupStat(m *metrics.MetricContext, path string, mp string) *PerCgroupStat {
	c := new(PerCgroupStat)
	c.m = m
	c.path = path
	rel, _ := filepath.Rel(mp, path)
	// initialize all metrics and register them
	c.prefix = "memstat.cgroup." + rel
	misc.InitializeMetrics(c, m, c.prefix, true)
	return c
}

// Unregister removes any entries to the metrics names in metrics context
func (s *PerCgroupStat) Unregister() {
	misc.UnregisterMetrics(s, s.m, s.prefix)
}

// Free returns free physical memory including cache
// Use soft_limit_in_bytes as upper bound or if not
// set use system memory
// NOT IMPLEMENTED YET
// rename to Free() when done
func (s *PerCgroupStat) free() float64 {
	return 0
}

// Usage returns physical memory in use; not including buffers/cached/sreclaimable
func (s *PerCgroupStat) Usage() float64 {
	return s.Rss.Get() + s.Mapped_file.Get()
}

// SoftLimit returns soft-limit for the cgroup
func (s *PerCgroupStat) SoftLimit() float64 {
	return s.Soft_Limit_In_Bytes.Get()
}

func (s *PerCgroupStat) Collect() {
	file, err := os.Open(s.path + "/" + "memory.stat")
	if err != nil {
		fmt.Println(err)
		return
	}

	d := map[string]*metrics.Gauge{}
	// Get all fields we care about
	r := reflect.ValueOf(s).Elem()
	typeOfT := r.Type()
	for i := 0; i < r.NumField(); i++ {
		f := r.Field(i)
		if f.Kind().String() == "ptr" && f.Type().Elem() == reflect.TypeOf(metrics.Gauge{}) {
			d[strings.ToLower(typeOfT.Field(i).Name)] =
				f.Interface().(*metrics.Gauge)
		}
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := regexp.MustCompile("[\\s]+").Split(scanner.Text(), 2)
		g, ok := d[strings.ToLower(f[0])]
		if ok {
			parseCgroupMemLine(g, f)
		}
	}

	s.Soft_Limit_In_Bytes.Set(
		float64(misc.ReadUintFromFile(
			s.path + "/" + "memory.soft_limit_in_bytes")))

	s.UsageInBytes.Set(s.Usage())
}

// Unexported functions
func parseCgroupMemLine(g *metrics.Gauge, f []string) {
	length := len(f)
	val := math.NaN()

	if length < 2 {
		goto fail
	}

	val = float64(misc.ParseUint(f[1]))
	g.Set(val)
	return

fail:
	g.Set(math.NaN())
	return
}
