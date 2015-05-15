// Package cpustat implements metrics collection related to CPU usage
package cpustat

import (
	"math"
	"time"
	"unsafe"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/misc"
)

// TODO: Per CPU stats - are they available?

/*
#include <mach/mach_init.h>
#include <mach/mach_error.h>
#include <mach/mach_host.h>
#include <mach/mach_port.h>
#include <mach/host_info.h>
*/
import "C"

// CPUStat represents metric information about all CPUs
type CPUStat struct {
	All *CPUStatPerCPU
	m   *metrics.MetricContext
}

type CPUStatPerCPU struct {
	User        *metrics.Counter
	UserLowPrio *metrics.Counter
	System      *metrics.Counter
	Idle        *metrics.Counter
	Total       *metrics.Counter // total ticks
	// Computed stats
	UsageCount     *metrics.Gauge
	UserSpaceCount *metrics.Gauge
	KernelCount    *metrics.Gauge
	TotalCount     *metrics.Gauge
}

func New(m *metrics.MetricContext, Step time.Duration) *CPUStat {
	c := new(CPUStat)
	c.All = CPUStatPerCPUNew(m, "cpu")
	c.m = m
	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			c.Collect()
		}
	}()
	return c
}

func (s *CPUStat) Collect() {

	// collect CPU stats for All cpus aggregated
	var cpuinfo C.host_cpu_load_info_data_t
	var hostinfo C.host_basic_info_data_t

	cpuloadnumber := C.mach_msg_type_number_t(C.HOST_CPU_LOAD_INFO_COUNT)
	hostnumber := C.mach_msg_type_number_t(C.HOST_BASIC_INFO_COUNT)
	host := C.mach_host_self()
	ret := C.host_statistics(C.host_t(host), C.HOST_CPU_LOAD_INFO,
		C.host_info_t(unsafe.Pointer(&cpuinfo)), &cpuloadnumber)

	if ret != C.KERN_SUCCESS {
		return
	}

	ret = C.host_info(C.host_t(host), C.HOST_BASIC_INFO,
		C.host_info_t(unsafe.Pointer(&hostinfo)), &hostnumber)
	if ret != C.KERN_SUCCESS {
		return
	}

	s.All.User.Set(uint64(cpuinfo.cpu_ticks[C.CPU_STATE_USER]))
	s.All.UserLowPrio.Set(uint64(cpuinfo.cpu_ticks[C.CPU_STATE_NICE]))
	s.All.System.Set(uint64(cpuinfo.cpu_ticks[C.CPU_STATE_SYSTEM]))
	s.All.Idle.Set(uint64(cpuinfo.cpu_ticks[C.CPU_STATE_IDLE]))

	s.All.Total.Set(uint64(cpuinfo.cpu_ticks[C.CPU_STATE_USER]) +
		uint64(cpuinfo.cpu_ticks[C.CPU_STATE_SYSTEM]) +
		uint64(cpuinfo.cpu_ticks[C.CPU_STATE_NICE]) +
		uint64(cpuinfo.cpu_ticks[C.CPU_STATE_IDLE]))
	s.All.UsageCount.Set(s.All.Usage())
	s.All.UserSpaceCount.Set(s.All.UserSpace())
	s.All.KernelCount.Set(s.All.Kernel())
	s.All.TotalCount.Set(float64(hostinfo.logical_cpu_max))
}

// Usage returns total work done over sampling interval
// Units: # of CPUs
func (o *CPUStat) Usage() float64 {
	return o.All.Usage() * o.Total()
}

// Userspace returns total work done over sampling interval in userspace
// Units: # of CPUs
// CPUs
func (o *CPUStat) UserSpace() float64 {
	return o.All.UserSpace() * o.Total()
}

// Kernel returns total work done over sampling interval in kernel
// CPUs
func (o *CPUStat) Kernel() float64 {
	return o.All.Kernel() * o.Total()
}

// Total returns maximum amount of work that can done over sampling interval
// Units: # of CPUs
func (o *CPUStat) Total() float64 {
	return o.All.TotalCount.Get()
}

// CPUStatPerCPUNew returns a struct representing counters for
// per CPU statistics
func CPUStatPerCPUNew(m *metrics.MetricContext, cpu string) *CPUStatPerCPU {
	o := new(CPUStatPerCPU)
	// initialize metrics and register
	// XXX: need to adopt it to similar to linux and pass
	// cpu name as argument when we are collecting per cpu
	// information
	misc.InitializeMetrics(o, m, "cpustat.cpu", true)
	return o
}

// Usage returns total work done in userspace + kernel
// Unit: # of logical CPUs
func (o *CPUStatPerCPU) Usage() float64 {
	u := o.User.ComputeRate()
	n := o.UserLowPrio.ComputeRate()
	s := o.System.ComputeRate()
	t := o.Total.ComputeRate()

	if u != math.NaN() && s != math.NaN() && t != math.NaN() && t > 0 {
		return (u + s + n) / t
	} else {
		return math.NaN()
	}
}

// Userspace returns total work done in userspace
// Unit: # of logical CPUs
func (o *CPUStatPerCPU) UserSpace() float64 {
	u := o.User.ComputeRate()
	n := o.UserLowPrio.ComputeRate()
	t := o.Total.ComputeRate()
	if u != math.NaN() && t != math.NaN() && n != math.NaN() && t > 0 {
		return (u + n) / t
	}
	return math.NaN()
}

// Kernel returns total work done in kernel
// Unit: # of logical CPUs
func (o *CPUStatPerCPU) Kernel() float64 {
	s := o.System.ComputeRate()
	t := o.Total.ComputeRate()
	if s != math.NaN() && t != math.NaN() && t > 0 {
		return (s / t)
	}
	return math.NaN()
}
