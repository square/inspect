// Copyright (c) 2014 Square, Inc

// Package memstat implements metrics collection related to Memory usage
package memstat

import (
	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/misc"
	"time"
	"unsafe"
)

/*
#include <mach/mach_init.h>
#include <mach/mach_error.h>
#include <mach/mach_host.h>
#include <mach/mach_port.h>
#include <mach/host_info.h>
#include <sys/types.h>
#include <sys/sysctl.h>
int64_t get_phys_memory() {
 	int mib[2];
    	int64_t phys_mem;
    	size_t length;

    	mib[0] = CTL_HW;
    	mib[1] = HW_MEMSIZE;
    	length = sizeof(int64_t);
    	sysctl(mib, 2, &phys_mem, &length, NULL, 0);

	return phys_mem;
}
*/
import "C"

type MemStat struct {
	Metrics *MemStatMetrics
	m       *metrics.MetricContext
}

func New(m *metrics.MetricContext, Step time.Duration) *MemStat {
	s := new(MemStat)
	s.Metrics = MemStatMetricsNew(m, Step)
	return s
}

// Free returns free memory
// Inactive lists may contain dirty pages
// Unfortunately there doesn't seem to be easy way
// to get that count
func (s *MemStat) Free() float64 {
	o := s.Metrics
	return o.Free.Get() + o.Inactive.Get() + o.Purgeable.Get()
}

// Usage returns physical memory in use
func (s *MemStat) Usage() float64 {
	o := s.Metrics
	return o.Total.Get() - s.Free()
}

// Usage returns total physical memory
func (s *MemStat) Total() float64 {
	o := s.Metrics
	return o.Total.Get()
}

type MemStatMetrics struct {
	Free      *metrics.Gauge
	Active    *metrics.Gauge
	Inactive  *metrics.Gauge
	Wired     *metrics.Gauge
	Purgeable *metrics.Gauge
	Total     *metrics.Gauge
	Pagesize  C.vm_size_t
}

func MemStatMetricsNew(m *metrics.MetricContext, Step time.Duration) *MemStatMetrics {
	c := new(MemStatMetrics)

	// initialize all gauges
	misc.InitializeMetrics(c, m, "memstat", true)

	host := C.mach_host_self()
	C.host_page_size(C.host_t(host), &c.Pagesize)

	// collect metrics every Step
	ticker := time.NewTicker(Step)
	go func() {
		for _ = range ticker.C {
			c.Collect()
		}
	}()

	return c
}

func (s *MemStatMetrics) Collect() {

	var meminfo C.vm_statistics64_data_t
	count := C.mach_msg_type_number_t(C.HOST_VM_INFO64_COUNT)

	host := C.mach_host_self()
	ret := C.host_statistics64(C.host_t(host), C.HOST_VM_INFO64,
		C.host_info_t(unsafe.Pointer(&meminfo)), &count)

	if ret != C.KERN_SUCCESS {
		return
	}

	s.Free.Set(float64(meminfo.free_count) * float64(s.Pagesize))
	s.Active.Set(float64(meminfo.active_count) * float64(s.Pagesize))
	s.Inactive.Set(float64(meminfo.inactive_count) * float64(s.Pagesize))
	s.Wired.Set(float64(meminfo.wire_count) * float64(s.Pagesize))
	s.Purgeable.Set(float64(meminfo.purgeable_count) * float64(s.Pagesize))
	s.Total.Set(float64(C.get_phys_memory()))

}
