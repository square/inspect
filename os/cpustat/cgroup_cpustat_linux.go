// Copyright (c) 2014 Square, Inc

package cpustat

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/misc"
)

/*
#include <unistd.h>
#include <sys/types.h>
*/
import "C"

var LINUX_TICKS_IN_SEC int = int(C.sysconf(C._SC_CLK_TCK))

type CgroupStat struct {
	Cgroups    map[string]*PerCgroupStat
	m          *metrics.MetricContext
	Mountpoint string
}

func NewCgroupStat(m *metrics.MetricContext, Step time.Duration) *CgroupStat {
	c := new(CgroupStat)
	c.m = m

	c.Cgroups = make(map[string]*PerCgroupStat, 1)

	mountpoint, err := misc.FindCgroupMount("cpu")
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
			// Delete references to this cgroup - so it can be free'd up
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
	// raw metrics
	Nr_periods     *metrics.Counter
	Nr_throttled   *metrics.Counter
	Throttled_time *metrics.Counter
	Cfs_period_us  *metrics.Gauge
	Cfs_quota_us   *metrics.Gauge
	Utime          *metrics.Counter
	Stime          *metrics.Counter
	// populate computed stats
	UsageCount     *metrics.Gauge
	UserspaceCount *metrics.Gauge
	KernelCount    *metrics.Gauge
	TotalCount     *metrics.Gauge
	ThrottleCount  *metrics.Gauge
	//
	m      *metrics.MetricContext
	path   string
	prefix string
}

func NewPerCgroupStat(m *metrics.MetricContext, path string, mp string) *PerCgroupStat {
	c := new(PerCgroupStat)
	c.m = m
	c.path = path
	// initialize all metrics and register them
	// XXX: Handle errors
	rel, _ := filepath.Rel(mp, path)
	c.prefix = "cpustat.cgroup." + rel
	misc.InitializeMetrics(c, m, c.prefix, true)
	return c
}

// Unregister any metrics from metrics context
func (s *PerCgroupStat) Unregister() {
	misc.UnregisterMetrics(s, s.m, s.prefix)
}

// Throttle returns amount of work that couldn't
// be done due to cgroup limits.
// Unit: Logical CPUs
func (s *PerCgroupStat) Throttle() float64 {
	throttled_sec := s.Throttled_time.ComputeRate()
	return (throttled_sec / (1 * 1000 * 1000 * 1000))
}

// Quota returns how many logical CPUs can be used by this cgroup
// Quota is adjusted to count of CPUs if it is not set
func (s *PerCgroupStat) Quota() float64 {
	quota := (s.Cfs_quota_us.Get() / s.Cfs_period_us.Get())
	nproc := float64(runtime.NumCPU())
	if quota <= 0 || quota > nproc {
		quota = nproc
	}
	return quota
}

// Usage returns total work done over sampling interval by processes
// in this cgroup in userspace+kernel
// Units: # of logical CPUs
func (s *PerCgroupStat) Usage() float64 {
	return s.UsageCount.Get()
}

// Userspace returns total work done over sampling interval by processes
// in this cgroup in userspace
// Units: # of logical CPUs
func (s *PerCgroupStat) Userspace() float64 {
	return s.UserspaceCount.Get()
}

// Kernel returns total work done over sampling interval by processes
// in this cgroup in kernel
// Units: # of logical CPUs
func (s *PerCgroupStat) Kernel() float64 {
	return s.KernelCount.Get()
}

// Collect reads cpu.stat for cgroups and per process cpu.stat
// entries for all processes in the cgroup
func (s *PerCgroupStat) Collect() {
	file, err := os.Open(s.path + "/" + "cpu.stat")
	defer file.Close()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := regexp.MustCompile("\\s+").Split(scanner.Text(), 2)

		if f[0] == "nr_periods" {
			s.Nr_periods.Set(misc.ParseUint(f[1]))
		}

		if f[0] == "nr_throttled" {
			s.Nr_throttled.Set(misc.ParseUint(f[1]))
		}

		if f[0] == "throttled_time" {
			s.Throttled_time.Set(misc.ParseUint(f[1]))
		}
	}

	s.Cfs_period_us.Set(
		float64(misc.ReadUintFromFile(
			s.path + "/" + "cpu.cfs_period_us")))
	// negative values of quota indicate no quota.
	// it is not possible for this variable to be zero
	s.Cfs_quota_us.Set(
		float64(misc.ReadUintFromFile(
			s.path + "/" + "cpu.cfs_quota_us")))

	// Calculate approximate cumulative CPU usage for all
	// processes within this cgroup by calculating difference
	// between sum number of ticks.
	// We reset between loops because PIDs within cgroup can
	// change and sum-counter from previous run can be
	// unreliable
	s.getCgroupCPUTimes()
	time.Sleep(time.Millisecond * 1000)
	s.getCgroupCPUTimes()
	// Expose summary metrics for easy json access
	s.UsageCount.Set(s.usage())
	s.UserspaceCount.Set(s.userspace())
	s.KernelCount.Set(s.kernel())
	s.TotalCount.Set(s.Quota())
	s.ThrottleCount.Set(s.Throttle())
	// Reset counters
	s.Utime.Reset()
	s.Stime.Reset()
}

// unexported

func (s *PerCgroupStat) usage() float64 {
	rate_per_sec := s.Utime.ComputeRate() + s.Stime.ComputeRate()
	return (rate_per_sec) / float64(LINUX_TICKS_IN_SEC)
}

func (s *PerCgroupStat) userspace() float64 {
	rate_per_sec := s.Utime.ComputeRate()
	return (rate_per_sec) / float64(LINUX_TICKS_IN_SEC)
}

func (s *PerCgroupStat) kernel() float64 {
	rate_per_sec := s.Stime.ComputeRate()
	return (rate_per_sec) / float64(LINUX_TICKS_IN_SEC)
}

func (s *PerCgroupStat) getCgroupCPUTimes() {
	// Compute user/system cpu times for all processes in this
	// cgroup
	var utime, stime uint64
	procsFd, err := os.Open(s.path + "/" + "cgroup.procs")
	defer procsFd.Close()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(procsFd)
	for scanner.Scan() {
		u, s := getCPUTimes(scanner.Text())
		utime += u
		stime += s
	}
	s.Utime.Set(utime)
	s.Stime.Set(stime)
}

func getCPUTimes(pid string) (uint64, uint64) {
	file, err := os.Open("/proc/" + pid + "/stat")
	defer file.Close()
	if err != nil {
		return 0, 0
	}

	var user, system uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := strings.Split(scanner.Text(), " ")
		user = misc.ParseUint(f[13])
		system = misc.ParseUint(f[14])
	}
	return user, system
}
