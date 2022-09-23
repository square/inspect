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

// LinuxTicksInSecond is number of ticks in a second as provided by
// SC_CLK_TCK sysconf
var linuxTicksInSecond = int(C.sysconf(C._SC_CLK_TCK))

// CgroupStat represents CPU related statistics gathered for all
// cgroups attached with non-default cpu subsystem
type CgroupStat struct {
	Cgroups    map[string]*PerCgroupStat
	m          *metrics.MetricContext
	Mountpoint string
}

// NewCgroupStat registers with metriccontext and starts collecting statistics
// for all cgroups every Step.
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

// Collect walks through cpu cgroup subsystem mount and collects cpu time
// spent in kernel/userspace for tasks belonging to non-default cgroup.
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

	for cgroup := range c.Cgroups {
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

// PerCgroupStat represents CPU related metrics for this particular cgroup under cpu subsystem
type PerCgroupStat struct {
	// raw metrics
	NrPeriods     *metrics.Counter
	NrThrottled   *metrics.Counter
	ThrottledTime *metrics.Counter
	CfsPeriodUs   *metrics.Gauge
	CfsQuotaUs    *metrics.Gauge
	Utime         *metrics.Counter
	Stime         *metrics.Counter
	// populate computed stats
	UsageCount     *metrics.Gauge
	UserspaceCount *metrics.Gauge
	KernelCount    *metrics.Gauge
	TotalCount     *metrics.Gauge
	ThrottleCount  *metrics.Gauge
	//
	m          *metrics.MetricContext
	path       string
	mountpoint string
	prefix     string
}

// NewPerCgroupStat registers with metricscontext for a particular cgroup
func NewPerCgroupStat(m *metrics.MetricContext, path, mp string) *PerCgroupStat {
	c := new(PerCgroupStat)
	c.m = m
	c.path = path
	c.mountpoint = mp
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
	throttledSec := s.ThrottledTime.ComputeRate()
	return (throttledSec / (1 * 1000 * 1000 * 1000))
}

// Quota returns how many logical CPUs can be used by this cgroup
// Quota is adjusted to count of CPUs if it is not set
func (s *PerCgroupStat) Quota() float64 {
	quota := (s.CfsQuotaUs.Get() / s.CfsPeriodUs.Get())
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
			s.NrPeriods.Set(misc.ParseUint(f[1]))
		}

		if f[0] == "nr_throttled" {
			s.NrThrottled.Set(misc.ParseUint(f[1]))
		}

		if f[0] == "throttled_time" {
			s.ThrottledTime.Set(misc.ParseUint(f[1]))
		}
	}

	s.CfsPeriodUs.Set(
		float64(misc.ReadUintFromFile(
			s.path + "/" + "cpu.cfs_period_us")))

	// Find the quota for the cgroup. If there is no limit (the value is
	// -1), we need to check the parent directory, recursively, until we
	// reach the root directory which is s.mountpoint.
	var quota float64
	path := s.path
	for {
		quota = float64(misc.ReadUintFromFile(
			path + "/" + "cpu.cfs_quota_us"))
		if quota > 0 {
			break
		}

		if path == s.mountpoint {
			quota = -1
			break
		}

		path = filepath.Dir(path)
	}
	s.CfsQuotaUs.Set(quota)

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
	ratePerSec := s.Utime.ComputeRate() + s.Stime.ComputeRate()
	return (ratePerSec) / float64(linuxTicksInSecond)
}

func (s *PerCgroupStat) userspace() float64 {
	ratePerSec := s.Utime.ComputeRate()
	return (ratePerSec) / float64(linuxTicksInSecond)
}

func (s *PerCgroupStat) kernel() float64 {
	ratePerSec := s.Stime.ComputeRate()
	return (ratePerSec) / float64(linuxTicksInSecond)
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
