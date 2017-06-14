package osmain

import (
	"fmt"
	"time"

	"github.com/gizak/termui"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/cpustat"
	"github.com/square/inspect/os/memstat"
	"github.com/square/inspect/os/misc"
	"github.com/square/inspect/os/pidstat"
)

type osIndependentStats struct {
	Cstat *cpustat.CPUStat
	Mstat *memstat.MemStat
	Procs *pidstat.ProcessStat
}

// Number of Pids(in future cgroups etc) to display for top-N metrics
const MaxEntries = 15

// DisplayWidgets represents various variables used for display
// Perhaps this belongs to main package
type DisplayWidgets struct {
	Summary           *termui.Par
	ProcessesByCPU    *termui.List
	ProcessesByMemory *termui.List
	ProcessesByIO     *termui.List
	DiskIOUsage       *termui.List
	FileSystemUsage   *termui.List
	InterfaceUsage    *termui.List
	CgroupsCPU        *termui.List
	CgroupsMem        *termui.List
	Problems          *termui.List
}

// Stats represents all statistics collected and printed by osmain
type Stats struct {
	CPUStat     *cpustat.CPUStat
	MemStat     *memstat.MemStat
	ProcessStat *pidstat.ProcessStat
	Problems    []string // various problems spotted
	OsSpecific  interface{}
}

// Register starts metrics collection for all available metrics
func Register(m *metrics.MetricContext, step time.Duration) *Stats {
	stats := new(Stats)
	// Collect cpu/memory/disk/perpid metrics
	stats.CPUStat = cpustat.New(m, step)
	stats.MemStat = memstat.New(m, step)
	p := pidstat.NewProcessStat(m, step)
	// Filter processes which have < 1% of a CPU or < 1% memory
	p.SetPidFilter(pidstat.PidFilterFunc(func(p *pidstat.PerProcessStat) bool {
		memUsagePct := (p.MemUsage() / stats.MemStat.Total()) * 100.0
		if p.CPUUsage() > 0.01 || memUsagePct > 1 {
			return true
		}
		return false
	}))
	stats.ProcessStat = p
	// register os dependent metrics
	// these could be specific to the OS (say cgroups)
	// or stats which are implemented not on all supported
	// platforms yet
	stats.OsSpecific = registerOsSpecific(m, step, stats)
	return stats
}

// Print inspects and prints various metrics collected started by Register
func (stats *Stats) Print(batchmode bool, layout *DisplayWidgets) {
	// deal with stats that are available on platforms
	memPctUsage := (stats.MemStat.Usage() / stats.MemStat.Total()) * 100
	cpuPctUsage := (stats.CPUStat.Usage() / stats.CPUStat.Total()) * 100
	cpuUserspacePctUsage := (stats.CPUStat.UserSpace() / stats.CPUStat.Total()) * 100
	cpuKernelPctUsage := (stats.CPUStat.Kernel() / stats.CPUStat.Total()) * 100
	// Top processes by usage
	procsByCPUUsage := stats.ProcessStat.ByCPUUsage()
	procsByMemUsage := stats.ProcessStat.ByMemUsage()
	// summary
	summaryLine := fmt.Sprintf(
		"total: cpu: %3.1f%% user: %3.1f%%, kernel: %3.1f%%, mem: %3.1f%%",
		cpuPctUsage, cpuUserspacePctUsage, cpuKernelPctUsage, memPctUsage)
	displayLine(batchmode, "summary", layout, summaryLine)
	if cpuPctUsage > 80.0 {
		stats.Problems = append(stats.Problems, "CPU usage is > 80%")
	}
	if cpuKernelPctUsage > 30.0 {
		stats.Problems = append(stats.Problems, "CPU usage in kernel is > 30%")
	}
	if memPctUsage > 80.0 {
		stats.Problems = append(stats.Problems, "Memory usage > 80%")
	}
	// Processes by cpu usage
	var cpu []string
	n := MaxEntries
	if len(procsByCPUUsage) < MaxEntries {
		n = len(procsByCPUUsage)
	}
	for i := 0; i < n; i++ {
		cpuUsagePct := (procsByCPUUsage[i].CPUUsage() / stats.CPUStat.Total()) * 100
		cpu = append(cpu, fmt.Sprintf("%5s %10s %10s %8s", fmt.Sprintf("%3.1f%%", cpuUsagePct),
			truncate(procsByCPUUsage[i].Comm(), 10),
			truncate(procsByCPUUsage[i].User(), 10),
			procsByCPUUsage[i].Pid()))
	}
	for i := n; i < MaxEntries; i++ {
		cpu = append(cpu, fmt.Sprintf("%5s %10s %10s %8s", "-", "-", "-", "-"))
	}
	displayList(batchmode, "cpu", layout, cpu)
	// Top processes by mem
	var mem []string
	n = MaxEntries
	if len(procsByMemUsage) < MaxEntries {
		n = len(procsByMemUsage)
	}
	for i := 0; i < n; i++ {
		mem = append(mem, fmt.Sprintf("%8s %10s %10s %8s",
			misc.ByteSize(procsByMemUsage[i].MemUsage()),
			truncate(procsByMemUsage[i].Comm(), 10),
			truncate(procsByMemUsage[i].User(), 10),
			procsByMemUsage[i].Pid()))
	}
	for i := n; i < MaxEntries; i++ {
		mem = append(mem, fmt.Sprintf("%8s %10s %10s %8s", "-", "-", "-", "-"))
	}
	displayList(batchmode, "memory", layout, mem)
	printOsSpecific(batchmode, layout, stats.OsSpecific)
	// finally deal with problems
	displayList(batchmode, "problem", layout, stats.Problems)
}

// few small helper functions
func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func displayLine(batchmode bool, name string, layout *DisplayWidgets, line string) {
	if !batchmode {
		switch name {
		case "summary":
			layout.Summary.Text = line
		}
	} else {
		fmt.Printf("%s:%s\n", name, line)
	}
}

func displayList(batchmode bool, name string, layout *DisplayWidgets, list []string) {
	if !batchmode {
		switch name {
		case "cpu":
			layout.ProcessesByCPU.Items = list
		case "cpu(cgroup)":
			layout.CgroupsCPU.Items = list
		case "memory":
			layout.ProcessesByMemory.Items = list
		case "memory(cgroup)":
			layout.CgroupsMem.Items = list
		case "io":
			layout.ProcessesByIO.Items = list
		case "interface":
			layout.InterfaceUsage.Items = list
		case "filesystem":
			layout.FileSystemUsage.Items = list
		case "diskio":
			layout.DiskIOUsage.Items = list
		case "problem":
			layout.Problems.Items = list
		}
	} else {
		for _, line := range list {
			fmt.Printf("%s:%s\n", name, line)
		}
	}
}
