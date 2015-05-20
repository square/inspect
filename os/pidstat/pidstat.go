// Package pidstat implements metrics collection per-PID like memory/cpu
package pidstat

import (
	"math"
	"sort"
)

// ProcessStatInterface defines common methods that all
// platform specific ProcessStat type must implement
type ProcessStatInterface interface {
	ByCPUUsage() []*PerProcessStat
	ByMemUsage() []*PerProcessStat
	SetPidFilter(PidFilterFunc)
}

var _ ProcessStatInterface = &ProcessStat{}

// PerProcessStatInterface defines common methods that
// all platform specific PerProcessStat types must
// implement
type PerProcessStatInterface interface {
	CPUUsage() float64
	MemUsage() float64
}

var _ PerProcessStatInterface = &PerProcessStat{}

// byCPUUsage implements sort.Interface for []*PerProcessStat based on
// the Usage() method
type byCPUUsage []*PerProcessStat

func (a byCPUUsage) Len() int           { return len(a) }
func (a byCPUUsage) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byCPUUsage) Less(i, j int) bool { return a[i].CPUUsage() > a[j].CPUUsage() }

// ByCPUUsage returns an slice of *PerProcessStat entries sorted
// by CPU usage
func (c *ProcessStat) ByCPUUsage() []*PerProcessStat {
	var v []*PerProcessStat
	for _, o := range c.Processes {
		if !math.IsNaN(o.CPUUsage()) {
			v = append(v, o)
		}
	}
	sort.Sort(byCPUUsage(v))
	return v
}

type byMemUsage []*PerProcessStat

func (a byMemUsage) Len() int           { return len(a) }
func (a byMemUsage) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byMemUsage) Less(i, j int) bool { return a[i].MemUsage() > a[j].MemUsage() }

// ByMemUsage returns an slice of *PerProcessStat entries sorted
// by Memory usage
func (c *ProcessStat) ByMemUsage() []*PerProcessStat {
	var v []*PerProcessStat
	for _, o := range c.Processes {
		if !math.IsNaN(o.MemUsage()) {
			v = append(v, o)
		}
	}
	sort.Sort(byMemUsage(v))
	return v
}

// PidFilterFunc represents a function that can be passed to PerProcessStat
// to express interest in tracking a particular process
type PidFilterFunc func(pidstat *PerProcessStat) (interested bool)

// Filter runs the user supplied filter function
func (f PidFilterFunc) Filter(pidstat *PerProcessStat) (interested bool) {
	return f(pidstat)
}

func defaultPidFilter(pidstat *PerProcessStat) bool {
	return true
}
