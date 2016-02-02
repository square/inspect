// Copyright (c) 2014 Square, Inc

package pidstat

import (
	"bufio"
	"errors"
	"io/ioutil"
	"math"
	"os"
	"os/user"
	"path"
	"regexp"
	"sort"
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

var linuxTicksInSec = int(C.sysconf(C._SC_CLK_TCK))
var pageSize = int(C.sysconf(C._SC_PAGESIZE))
var root = "/" // to make testing easy

// ProcessStat represents per-process cpu usage statistics
type ProcessStat struct {
	Processes map[string]*PerProcessStat
	m         *metrics.MetricContext
	x         []*PerProcessStat
	filter    PidFilterFunc
}

// NewProcessStat registers with metriccontext and collects per-process
// cpu statistics every Step
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
	for i := range c.x {
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

// SetPidFilter takes a PidFilterFunc and applies it as a filter
// to reduce number of processes to keep track of.
func (s *ProcessStat) SetPidFilter(filter PidFilterFunc) {
	s.filter = filter
	return
}

// Return list of processes sorted by IO
type byIOUsage []*PerProcessStat

func (a byIOUsage) Len() int           { return len(a) }
func (a byIOUsage) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byIOUsage) Less(i, j int) bool { return a[i].IOUsage() > a[j].IOUsage() }

// ByIOUsage returns an slice of *PerProcessStat entries sorted
// by Memory usage
func (s *ProcessStat) ByIOUsage() []*PerProcessStat {
	var v []*PerProcessStat
	for _, o := range s.Processes {
		if !math.IsNaN(o.IOUsage()) {
			v = append(v, o)
		}
	}
	sort.Sort(byIOUsage(v))
	return v
}

// CPUUsagePerCgroup returns cumulative CPU usage by cgroup
func (s *ProcessStat) CPUUsagePerCgroup(cgroup string) float64 {
	var ret float64
	if !path.IsAbs(cgroup) {
		cgroup = "/" + cgroup
	}

	for _, o := range s.Processes {
		if (o.Cgroup("cpu") == cgroup) && !math.IsNaN(o.CPUUsage()) {
			ret += o.CPUUsage()
		}
	}
	return ret
}

// MemUsagePerCgroup returns cumulative Memory usage by cgroup
func (s *ProcessStat) MemUsagePerCgroup(cgroup string) float64 {
	var ret float64
	if !path.IsAbs(cgroup) {
		cgroup = "/" + cgroup
	}
	for _, o := range s.Processes {
		if (o.Cgroup("memory") == cgroup) && !math.IsNaN(o.MemUsage()) {
			ret += o.MemUsage()
		}
	}
	return ret
}

// Collect walks through /proc and updates stats
// Collect is usually called internally based on
// parameters passed via metric context
func (s *ProcessStat) Collect() {
	h := s.Processes
	for _, v := range h {
		v.Metrics.dead = true
	}

	pids, err := ioutil.ReadDir(root + "proc")
	if err != nil {
		return
	}

	// scan 1024 processes at once to pick out the ones
	// that are interesting

	for startIdx := 0; startIdx < len(pids); startIdx += 1024 {
		endIdx := startIdx + 1024
		if endIdx > len(pids) {
			endIdx = len(pids)
		}

		for _, pidstat := range s.x {
			pidstat.Reset("?")
		}

		s.scanProc(&pids, startIdx, endIdx)
		time.Sleep(time.Millisecond * 1000)
		s.scanProc(&pids, startIdx, endIdx)

		for i, pidstat := range s.x {
			if s.filter(pidstat) {
				h[pidstat.Pid()] = pidstat
				pidstat.Metrics.Register() // forces registration with new name
				s.x[i] = NewPerProcessStat(s.m, "")
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
func (s *ProcessStat) scanProc(pids *[]os.FileInfo, startIdx int, endIdx int) {

	pidre := regexp.MustCompile("^\\d+")
	for i := startIdx; i < endIdx; i++ {
		f := (*pids)[i]
		p := f.Name()
		if f.IsDir() && pidre.MatchString(p) {
			pidstat := s.x[i%1024]
			pidstat.Metrics.Pid = p
			pidstat.Metrics.Collect()
		}
	}
}

// PerProcessStat represents per process statistics and methods.
type PerProcessStat struct {
	Metrics *PerProcessStatMetrics
	m       *metrics.MetricContext
}

// NewPerProcessStat registers with metriccontext for single process
func NewPerProcessStat(m *metrics.MetricContext, p string) *PerProcessStat {
	s := new(PerProcessStat)
	s.m = m
	s.Metrics = NewPerProcessStatMetrics(m, p)
	return s
}

// Reset initializes all usage counters to zeros and the instance
// can be reused.
func (s *PerProcessStat) Reset(p string) {
	s.Metrics.Reset(p)
}

// CPUUsage returns amount of work done by this process in kernel/user
// Unit: # of logical CPUs
func (s *PerProcessStat) CPUUsage() float64 {
	o := s.Metrics
	ratePerSec := (o.Utime.ComputeRate() + o.Stime.ComputeRate())
	return ratePerSec / float64(linuxTicksInSec)
}

// MemUsage returns amount of memory resident for this process in bytes.
func (s *PerProcessStat) MemUsage() float64 {
	o := s.Metrics
	return o.Rss.Get() * float64(pageSize)
}

// IOUsage returns cumulative bytes read/written by this process (bytes/sec)
func (s *PerProcessStat) IOUsage() float64 {
	o := s.Metrics
	return o.IOReadBytes.ComputeRate() + o.IOWriteBytes.ComputeRate()
}

// Pid returns the pid for this process
func (s *PerProcessStat) Pid() string {
	return s.Metrics.Pid
}

// Comm returns the command used to run for this process
func (s *PerProcessStat) Comm() string {
	file, err := os.Open(root + "proc/" + s.Metrics.Pid + "/stat")
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

// Euid returns the effective uid for this process
func (s *PerProcessStat) Euid() (string, error) {
	file, err := os.Open(root + "proc/" + s.Metrics.Pid + "/status")
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

// Egid returns the effective gid for this process
func (s *PerProcessStat) Egid() (string, error) {
	file, err := os.Open(root + "proc/" + s.Metrics.Pid + "/status")
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

// User returns the username for the process - looked up by effective uid
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

// Cmdline returns the complete command line used to invoke this process
func (s *PerProcessStat) Cmdline() string {
	content, err := ioutil.ReadFile(root + "proc/" + s.Metrics.Pid + "/cmdline")
	if err != nil {
		return string(content)
	}

	return ""
}

// Cgroup returns the name of the cgroup for this process for the input
// cgroup subsystem
func (s *PerProcessStat) Cgroup(subsys string) string {
	file, err := os.Open(root + "proc/" + s.Metrics.Pid + "/cgroup")
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

// PerProcessStatMetrics represents metrics for the per process
// stats collection
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

// NewPerProcessStatMetrics registers with metricscontext
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

// Reset resets all counters and gauges to original values
func (s *PerProcessStatMetrics) Reset(pid string) {
	s.Pid = pid
	s.Utime.Reset()
	s.Stime.Reset()
	s.Rss.Reset()
	s.IOReadBytes.Reset()
	s.IOWriteBytes.Reset()
}

// Collect collects per process CPU/Memory/IO metrics
func (s *PerProcessStatMetrics) Collect() {

	file, err := os.Open(root + "proc/" + s.Pid + "/stat")
	defer file.Close()

	if err != nil {
		return
	}

	scanner := bufio.NewScanner(file)
	// command names can have spaces in them and are captured with ()
	r := regexp.MustCompile("(\\d+)\\s\\((.*)\\)\\s(.*)")
	for scanner.Scan() {
		parts := r.FindStringSubmatch(scanner.Text())
		f := strings.Split(parts[3], " ")
		s.Utime.Set(misc.ParseUint(f[11]))
		s.Stime.Set(misc.ParseUint(f[12]))
		s.Rss.Set(float64(misc.ParseUint(f[21])))
	}

	// collect IO metrics
	// only works if we are superuser on Linux
	file, err = os.Open(root + "proc/" + s.Pid + "/io")
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
