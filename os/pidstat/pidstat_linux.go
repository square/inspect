// Copyright (c) 2014 Square, Inc

package pidstat

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/misc"
	"io/ioutil"
	"math"
	"os"
	"os/user"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

/*
#include <unistd.h>
#include <sys/types.h>
*/
import "C"

var LINUX_TICKS_IN_SEC int = int(C.sysconf(C._SC_CLK_TCK))
var PAGESIZE int = int(C.sysconf(C._SC_PAGESIZE))
var _ = fmt.Println

// NewProcessStat allocates a new ProcessStat object
// Arguments:
// m - *metricContext
// Step - time.Duration

type ProcessStat struct {
	Processes map[string]*PerProcessStat
	m         *metrics.MetricContext
	x         []*PerProcessStat
	filter    PidFilterFunc
}

// Collects metrics every Step seconds
// Sleeps an additional 1s for every 1024 processes
// TODO: Implement better heuristics to manage load
//   * Collect metrics for newer processes at faster rate
//   * Slower rate for processes with neglible rate?

func NewProcessStat(m *metrics.MetricContext, Step time.Duration) *ProcessStat {
	c := new(ProcessStat)
	c.m = m

	c.Processes = make(map[string]*PerProcessStat, 64)

	// pool for PerProcessStat objects
	// stupid trick to avoid depending on GC to free up
	// temporary pool
	c.x = make([]*PerProcessStat, 1024)
	for i, _ := range c.x {
		c.x[i] = NewPerProcessStat(m, "")
	}

	// Assign a default filter for pids
	c.filter = PidFilterFunc(defaultPidFilter)

	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			c.Collect()
		}
	}()

	return c
}

func (s *ProcessStat) SetPidFilter(filter PidFilterFunc) {
	s.filter = filter
	return
}

// Return list of processes sorted by IO
type ByIOUsage []*PerProcessStat

func (a ByIOUsage) Len() int           { return len(a) }
func (a ByIOUsage) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByIOUsage) Less(i, j int) bool { return a[i].IOUsage() > a[j].IOUsage() }

// ByIOUsage() returns an slice of *PerProcessStat entries sorted
// by Memory usage
func (c *ProcessStat) ByIOUsage() []*PerProcessStat {
	v := make([]*PerProcessStat, 0)
	for _, o := range c.Processes {
		if !math.IsNaN(o.IOUsage()) {
			v = append(v, o)
		}
	}
	sort.Sort(ByIOUsage(v))
	return v
}

// CPUUsagePerCgroup returns cumulative CPU usage by cgroup
func (c *ProcessStat) CPUUsagePerCgroup(cgroup string) float64 {
	var ret float64
	if !path.IsAbs(cgroup) {
		cgroup = "/" + cgroup
	}

	for _, o := range c.Processes {
		if (o.Cgroup("cpu") == cgroup) && !math.IsNaN(o.CPUUsage()) {
			ret += o.CPUUsage()
		}
	}
	return ret
}

// MemUsagePerCgroup(cgroup_name) returns cumulative Memory usage by cgroup
func (c *ProcessStat) MemUsagePerCgroup(cgroup string) float64 {
	var ret float64
	if !path.IsAbs(cgroup) {
		cgroup = "/" + cgroup
	}
	for _, o := range c.Processes {
		if (o.Cgroup("memory") == cgroup) && !math.IsNaN(o.MemUsage()) {
			ret += o.MemUsage()
		}
	}
	return ret
}

// Collect walks through /proc and updates stats
// Collect is usually called internally based on
// parameters passed via metric context
func (c *ProcessStat) Collect() {
	h := c.Processes
	for _, v := range h {
		v.Metrics.dead = true
	}

	pids, err := ioutil.ReadDir("/proc")
	if err != nil {
		return
	}

	// scan 1024 processes at once to pick out the ones
	// that are interesting

	for start_idx := 0; start_idx < len(pids); start_idx += 1024 {
		end_idx := start_idx + 1024
		if end_idx > len(pids) {
			end_idx = len(pids)
		}

		for _, pidstat := range c.x {
			pidstat.Reset("?")
		}

		c.scanProc(&pids, start_idx, end_idx)
		time.Sleep(time.Millisecond * 1000)
		c.scanProc(&pids, start_idx, end_idx)

		for i, pidstat := range c.x {
			if c.filter(pidstat) {
				h[pidstat.Pid()] = pidstat
				pidstat.Metrics.Register() // forces registration with new name
				c.x[i] = NewPerProcessStat(c.m, "")
				pidstat.Metrics.dead = false
			}
		}
	}

	// remove dead processes
	for k, v := range h {
		if v.Metrics.dead {
			v.Metrics.Unregister()
			delete(h, k)
		}
	}
}

// unexported
func (c *ProcessStat) scanProc(pids *[]os.FileInfo, start_idx int, end_idx int) {

	pidre := regexp.MustCompile("^\\d+")
	for i := start_idx; i < end_idx; i++ {
		f := (*pids)[i]
		p := f.Name()
		if f.IsDir() && pidre.MatchString(p) {
			pidstat := c.x[i%1024]
			pidstat.Metrics.Pid = p
			pidstat.Metrics.Collect()
		}
	}
}

// Per Process functions
type PerProcessStat struct {
	Metrics *PerProcessStatMetrics
	m       *metrics.MetricContext
}

func NewPerProcessStat(m *metrics.MetricContext, p string) *PerProcessStat {
	s := new(PerProcessStat)
	s.m = m
	s.Metrics = NewPerProcessStatMetrics(m, p)
	return s
}

func (s *PerProcessStat) Reset(p string) {
	s.Metrics.Reset(p)
}

// CPUUsage returns amount of work done by this process in kernel/user
// Unit: # of logical CPUs
func (s *PerProcessStat) CPUUsage() float64 {
	o := s.Metrics
	rate_per_sec := (o.Utime.ComputeRate() + o.Stime.ComputeRate())
	return rate_per_sec / float64(LINUX_TICKS_IN_SEC)
}

func (s *PerProcessStat) MemUsage() float64 {
	o := s.Metrics
	return o.Rss.Get() * float64(PAGESIZE)
}

func (s *PerProcessStat) IOUsage() float64 {
	o := s.Metrics
	return o.IOReadBytes.ComputeRate() + o.IOWriteBytes.ComputeRate()
}

func (s *PerProcessStat) Pid() string {
	return s.Metrics.Pid
}

func (s *PerProcessStat) Comm() string {
	file, err := os.Open("/proc/" + s.Metrics.Pid + "/stat")
	defer file.Close()

	if err != nil {
		return ""
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := strings.Split(scanner.Text(), " ")
		return strings.Trim(f[1], "()")
	}

	return ""
}

func (s *PerProcessStat) Euid() (string, error) {
	file, err := os.Open("/proc/" + s.Metrics.Pid + "/status")
	defer file.Close()

	if err != nil {
		return "", err
	}

	splitre := regexp.MustCompile("\\s+")
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := splitre.Split(scanner.Text(), -1)
		if f[0] == "Uid:" {
			return f[2], nil // effective uid
		}
	}

	return "", errors.New("unable to determine euid")
}

func (s *PerProcessStat) Egid() (string, error) {
	file, err := os.Open("/proc/" + s.Metrics.Pid + "/status")
	defer file.Close()

	if err != nil {
		return "", err
	}

	splitre := regexp.MustCompile("\\s+")
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := splitre.Split(scanner.Text(), -1)
		if f[0] == "Gid:" {
			return f[2], nil // effective gid
		}
	}

	return "", errors.New("unable to determine egid")
}

func (s *PerProcessStat) User() string {
	euid, err := s.Euid()

	if err != nil {
		return "?"
	}

	u, err := user.LookupId(euid)
	if err != nil {
		return "?"
	}

	return u.Username
}

func (s *PerProcessStat) Cmdline() string {
	content, err := ioutil.ReadFile("/proc/" + s.Metrics.Pid + "/cmdline")
	if err != nil {
		return string(content)
	}

	return ""
}

func (s *PerProcessStat) Cgroup(subsys string) string {
	file, err := os.Open("/proc/" + s.Metrics.Pid + "/cgroup")
	defer file.Close()

	if err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			f := strings.Split(scanner.Text(), ":")
			if f[1] == subsys {
				return f[2]
			}
		}
	}

	return "/"
}

type PerProcessStatMetrics struct {
	Pid          string
	Utime        *metrics.Counter
	Stime        *metrics.Counter
	Rss          *metrics.Gauge
	IOReadBytes  *metrics.Counter
	IOWriteBytes *metrics.Counter
	m            *metrics.MetricContext
	dead         bool
}

func NewPerProcessStatMetrics(m *metrics.MetricContext, pid string) *PerProcessStatMetrics {
	s := new(PerProcessStatMetrics)
	s.Pid = pid
	s.m = m

	// initialize all metrics but do NOT register them
	// registration happens if the objects pass user
	// supplied filter
	misc.InitializeMetrics(s, m, "IGNORE", false)

	return s
}

// Register metrics with metric context
func (s *PerProcessStatMetrics) Register() {
	prefix := "pidstat.pid" + s.Pid
	s.m.Register(s.Utime, prefix+"."+"Utime")
	s.m.Register(s.Stime, prefix+"."+"Stime")
	s.m.Register(s.Rss, prefix+"."+"Rss")
	s.m.Register(s.IOReadBytes, prefix+"."+"IOReadBytes")
	s.m.Register(s.IOWriteBytes, prefix+"."+"IOWriteBytes")
}

// Unregister metrics with metriccontext
func (s *PerProcessStatMetrics) Unregister() {
	prefix := "pidstat.pid" + s.Pid
	s.m.Unregister(s.Utime, prefix+"."+"Utime")
	s.m.Unregister(s.Stime, prefix+"."+"Stime")
	s.m.Unregister(s.Rss, prefix+"."+"Rss")
	s.m.Unregister(s.IOReadBytes, prefix+"."+"IOReadBytes")
	s.m.Unregister(s.IOWriteBytes, prefix+"."+"IOWriteBytes")
}

func (s *PerProcessStatMetrics) Reset(pid string) {
	s.Pid = pid
	s.Utime.Reset()
	s.Stime.Reset()
	s.Rss.Reset()
	s.IOReadBytes.Reset()
	s.IOWriteBytes.Reset()
}

// Collect() collects per process CPU/Memory/IO metrics
func (s *PerProcessStatMetrics) Collect() {

	file, err := os.Open("/proc/" + s.Pid + "/stat")
	defer file.Close()

	if err != nil {
		return
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		f := strings.Split(scanner.Text(), " ")
		s.Utime.Set(misc.ParseUint(f[13]))
		s.Stime.Set(misc.ParseUint(f[14]))
		s.Rss.Set(float64(misc.ParseUint(f[23])))
	}

	// collect IO metrics
	// only works if we are superuser on Linux
	file, err = os.Open("/proc/" + s.Pid + "/io")
	defer file.Close()

	if err != nil {
		return
	}

	scanner = bufio.NewScanner(file)
	for scanner.Scan() {
		f := strings.Split(scanner.Text(), " ")
		switch f[0] {
		case "read_bytes:":
			s.IOReadBytes.Set(misc.ParseUint(f[1]))
		case "write_bytes:":
			s.IOWriteBytes.Set(misc.ParseUint(f[1]))
		}
	}
}
