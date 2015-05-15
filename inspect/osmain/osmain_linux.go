// Copyright (c) 2014 Square, Inc
// +build linux

package osmain

import (
	"fmt"
	//"math"
	"path/filepath"
	"sort"
	"time"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/cpustat"
	"github.com/square/inspect/os/diskstat"
	"github.com/square/inspect/os/fsstat"
	"github.com/square/inspect/os/interfacestat"
	"github.com/square/inspect/os/loadstat"
	"github.com/square/inspect/os/memstat"
	"github.com/square/inspect/os/misc"
	"github.com/square/inspect/os/pidstat"
	"github.com/square/inspect/os/tcpstat"
	"github.com/mgutz/ansi"
)

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

type LinuxStats struct {
	dstat    *diskstat.DiskStat
	fsstat   *fsstat.FSStat
	ifstat   *interfacestat.InterfaceStat
	tcpstat  *tcpstat.TCPStat
	cgMem    *memstat.CgroupStat
	cgCPU    *cpustat.CgroupStat
	procs    *pidstat.ProcessStat
	cstat    *cpustat.CPUStat
	mstat    *memstat.MemStat
	loadstat *loadstat.LoadStat
}

// RegisterOsDependent registers OS dependent statistics
func RegisterOsDependent(m *metrics.MetricContext, step time.Duration,
	d *OsIndependentStats) *LinuxStats {

	s := new(LinuxStats)
	s.dstat = diskstat.New(m, step)
	s.fsstat = fsstat.New(m, step)
	s.ifstat = interfacestat.New(m, step)
	s.tcpstat = tcpstat.New(m, step)
	s.loadstat = loadstat.New(m, step)
	s.procs = d.Procs // grab it because we need to for per cgroup cpu usage
	s.cstat = d.Cstat
	s.mstat = d.Mstat
	s.cgMem = memstat.NewCgroupStat(m, step)
	s.cgCPU = cpustat.NewCgroupStat(m, step)

	return s
}

// PrintOsDependent prints OS dependent statistics
func PrintOsDependent(s *LinuxStats, batchmode bool) {

	var problems []string
	var keys []string

	procsByUsage := s.procs.ByIOUsage()
	n := 3
	if len(procsByUsage) < n {
		n = len(procsByUsage)
	}
	fmt.Println("\033[7m")
	fmt.Println(fmt.Sprintf("%8s %10s %10s %8s", "IO /s", "COMMAND", "USER", "PID"), ansi.ColorCode("reset"))
	for i := 0; i < n; i++ {
		fmt.Println(fmt.Sprintf("%8s %10s %10s %8s",
			misc.ByteSize(procsByUsage[i].IOUsage()),
			truncate(procsByUsage[i].Comm(), 10),
			truncate(procsByUsage[i].User(), 10),
			procsByUsage[i].Pid()))
	}
	for i := n; i < 3; i++ {
		fmt.Println(fmt.Sprintf("%8s %10s %10s %8s", "-", "-", "-", "-"))
	}

	type cgStat struct {
		cpu *cpustat.PerCgroupStat
		mem *memstat.PerCgroupStat
	}
	// Print top-5 filesystem/diskio usage
	// disk stats
	fmt.Println("\033[7m")
	fmt.Println(fmt.Sprintf("%6s %5s | %20s %6s %6s", "DISK", "IO", "FILESYSTEM", "USAGE", "INODES"), ansi.ColorCode("reset"))

	diskIOByUsage := s.dstat.ByUsage()
	fsByUsage := s.fsstat.ByUsage()
	for i := 0; i < 5; i++ {
		diskName := "-"
		diskIO := 0.0
		fsName := "-"
		fsUsage := 0.0
		fsInodes := 0.0
		if len(diskIOByUsage) > i {
			d := diskIOByUsage[i]
			diskName = d.Name
			diskIO = d.Usage()
		}
		if len(fsByUsage) > i {
			f := fsByUsage[i]
			fsName = f.Name
			fsUsage = f.Usage()
			fsInodes = f.FileUsage()
		}
		fmt.Printf("%6s %5s | %20s %6s %6s\n",
			diskName,
			fmt.Sprintf("%3.1f%% ", diskIO),
			truncate(fsName, 20),
			fmt.Sprintf("%3.1f%% ", fsUsage),
			fmt.Sprintf("%3.1f%% ", fsInodes))
	}
	// Detect potential problems for disk/fs
	for _, d := range diskIOByUsage {
		if d.Usage() > 75.0 {
			problems = append(problems,
				fmt.Sprintf("Disk IO usage on (%v): %3.1f%%", d.Name, d.Usage()))
		}
	}
	for _, fs := range fsByUsage {
		if fs.Usage() > 90.0 {
			problems = append(problems,
				fmt.Sprintf("FS block usage on (%v): %3.1f%%", fs.Name, fs.Usage()))
		}
		if fs.FileUsage() > 90.0 {
			problems = append(problems,
				fmt.Sprintf("FS inode usage on (%v): %3.1f%%", fs.Name, fs.FileUsage()))
		}
	}
	// Interface usage statistics
	fmt.Println("\033[7m")
	fmt.Println(fmt.Sprintf("%10s %8s %8s", "IFACE", "TX", "RX"), ansi.ColorCode("reset"))
	interfaceByUsage := s.ifstat.ByUsage()
	for i := 0; i < 5; i++ {
		name := "-"
		var rx misc.BitSize
		var tx misc.BitSize
		if len(interfaceByUsage) > i {
			iface := interfaceByUsage[i]
			name = truncate(iface.Name, 10)
			rx = misc.BitSize(iface.TXBandwidth())
			tx = misc.BitSize(iface.RXBandwidth())
		}
		fmt.Printf("%10s %8s %8s\n", name, rx, tx)
	}
	for _, iface := range interfaceByUsage {
		if iface.TXBandwidthUsage() > 75.0 {
			problems = append(problems,
				fmt.Sprintf("TX bandwidth usage on (%v): %3.1f%%",
					iface.Name, iface.TXBandwidthUsage()))
		}
		if iface.RXBandwidthUsage() > 75.0 {
			problems = append(problems,
				fmt.Sprintf("RX bandwidth usage on (%v): %3.1f%%",
					iface.Name, iface.RXBandwidthUsage()))
		}
	}

	// Cgroup stats
	fmt.Println("\033[7m")
	fmt.Println(fmt.Sprintf("%20s %5s %5s %5s %8s %5s %8s %5s", "CGROUP",
		"CPU", "QUOTA", "Q-PCT", "THROTTLE",
		"MEM", "QUOTA", "Q-PCT"), ansi.ColorCode("reset"))
	keys = keys[:0]
	// so much for printing cpu/mem stats for cgroup together
	cgStats := make(map[string]*cgStat)
	for name, mem := range s.cgMem.Cgroups {
		name, _ = filepath.Rel(s.cgMem.Mountpoint, name)
		_, ok := cgStats[name]
		if !ok {
			cgStats[name] = new(cgStat)
		}
		cgStats[name].mem = mem
	}
	for name, cpu := range s.cgCPU.Cgroups {
		name, _ = filepath.Rel(s.cgCPU.Mountpoint, name)
		_, ok := cgStats[name]
		if !ok {
			cgStats[name] = new(cgStat)
		}
		cgStats[name].cpu = cpu
	}
	for k := range cgStats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, name := range keys {
		v := cgStats[name]
		cpuUsagePct := 0.0
		cpuQuota := 0.0
		cpuQuotaPct := 0.0
		cpuThrottle := 0.0
		memUsagePct := 0.0
		memQuota := 0.0
		memQuotaPct := 0.0
		if v.cpu != nil {
			cpuUsagePct = (v.cpu.Usage() / s.cstat.Total()) * 100
			cpuQuotaPct = (v.cpu.Usage() / v.cpu.Quota()) * 100
			cpuThrottle = v.cpu.Throttle() * 100
			cpuQuota = v.cpu.Quota()
			if cpuThrottle > 0.5 {
				problems =
					append(problems,
						fmt.Sprintf("CPU throttling on cgroup(%s): %3.1f%%", name, cpuThrottle))
			}
		}
		if v.mem != nil {
			memUsagePct = (v.mem.Usage() / s.mstat.Total()) * 100
			memQuota = v.mem.SoftLimit()
			memQuotaPct = (v.mem.Usage() / v.mem.SoftLimit()) * 100
		}
		fmt.Printf("%20s %5s %5s %5s %8s %5s %8s %5s\n", truncate(name, 20),
			fmt.Sprintf("%3.1f%%", cpuUsagePct),
			fmt.Sprintf("%3.1f", cpuQuota),
			fmt.Sprintf("%3.1f%%", cpuQuotaPct),
			fmt.Sprintf("%3.1f%%", cpuThrottle),
			fmt.Sprintf("%3.1f%%", memUsagePct),
			misc.ByteSize(memQuota),
			fmt.Sprintf("%3.1f%%", memQuotaPct))
	}

	// finally print all potential problems
	fmt.Println("---")
	for i := range problems {
		msg := problems[i]
		if !batchmode {
			msg = ansi.Color(msg, "red")
		}
		fmt.Println("Problem: ", msg)
	}
}
