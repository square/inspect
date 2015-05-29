// Copyright (c) 2014 Square, Inc
// +build linux

package osmain

import (
	"fmt"
	//"math"
	"log"
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
	"github.com/square/inspect/os/tcpstat"
)

type linuxStats struct {
	osind    *Stats
	dstat    *diskstat.DiskStat
	fsstat   *fsstat.FSStat
	ifstat   *interfacestat.InterfaceStat
	tcpstat  *tcpstat.TCPStat
	cgMem    *memstat.CgroupStat
	cgCPU    *cpustat.CgroupStat
	loadstat *loadstat.LoadStat
}

// RegisterOsSpecific registers OS dependent statistics
func registerOsSpecific(m *metrics.MetricContext, step time.Duration,
	osind *Stats) *linuxStats {
	s := new(linuxStats)
	s.osind = osind
	s.dstat = diskstat.New(m, step)
	s.fsstat = fsstat.New(m, step)
	s.ifstat = interfacestat.New(m, step)
	s.tcpstat = tcpstat.New(m, step)
	s.loadstat = loadstat.New(m, step)
	s.cgMem = memstat.NewCgroupStat(m, step)
	s.cgCPU = cpustat.NewCgroupStat(m, step)
	return s
}

// PrintOsSpecific prints OS dependent statistics
func printOsSpecific(batchmode bool, layout *DisplayWidgets, v interface{}) {
	stats, ok := v.(*linuxStats)
	if !ok {
		log.Fatalf("Type assertion failed on printOsSpecific")
	}
	// Top N processes sorted by IO usage - requires root
	procsByUsage := stats.osind.ProcessStat.ByIOUsage()
	n := DisplayPidCount
	if len(procsByUsage) < n {
		n = len(procsByUsage)
	}
	var io []string
	for i := 0; i < n; i++ {
		io = append(io, fmt.Sprintf("%8s/s %10s %10s %8s",
			misc.ByteSize(procsByUsage[i].IOUsage()),
			truncate(procsByUsage[i].Comm(), 10),
			truncate(procsByUsage[i].User(), 10),
			procsByUsage[i].Pid()))
	}
	for i := n; i < DisplayPidCount; i++ {
		io = append(io, fmt.Sprintf("%8s/s %10s %10s %8s", "-", "-", "-", "-"))
	}
	displayList(batchmode, "io", layout, io)
	// Print top-N diskIO usage
	// disk stats
	diskIOByUsage := stats.dstat.ByUsage()
	var diskio []string
	// TODO(syamp): remove magic number
	for i := 0; i < 5; i++ {
		diskName := "-"
		diskIO := 0.0
		if len(diskIOByUsage) > i {
			d := diskIOByUsage[i]
			diskName = d.Name
			diskIO = d.Usage()
		}
		diskio = append(diskio, fmt.Sprintf("%6s %5s", diskName, fmt.Sprintf("%3.1f%% ", diskIO)))
	}
	displayList(batchmode, "diskio", layout, diskio)
	// Print top-N File system  usage
	// disk stats
	fsByUsage := stats.fsstat.ByUsage()
	var fs []string
	// TODO(syamp): remove magic number
	for i := 0; i < 5; i++ {
		fsName := "-"
		fsUsage := 0.0
		fsInodes := 0.0
		if len(fsByUsage) > i {
			f := fsByUsage[i]
			fsName = f.Name
			fsUsage = f.Usage()
			fsInodes = f.FileUsage()
		}
		fs = append(fs, fmt.Sprintf("%20s %6s i:%6s", truncate(fsName, 20),
			fmt.Sprintf("%3.1f%%", fsUsage),
			fmt.Sprintf("%3.1f%%", fsInodes)))
	}
	displayList(batchmode, "filesystem", layout, fs)
	// Detect potential problems for disk/fs
	for _, d := range diskIOByUsage {
		if d.Usage() > 75.0 {
			stats.osind.Problems = append(stats.osind.Problems,
				fmt.Sprintf("Disk IO usage on (%v): %3.1f%%", d.Name, d.Usage()))
		}
	}
	for _, fs := range fsByUsage {
		if fs.Usage() > 90.0 {
			stats.osind.Problems = append(stats.osind.Problems,
				fmt.Sprintf("FS block usage on (%v): %3.1f%%", fs.Name, fs.Usage()))
		}
		if fs.FileUsage() > 90.0 {
			stats.osind.Problems = append(stats.osind.Problems,
				fmt.Sprintf("FS inode usage on (%v): %3.1f%%", fs.Name, fs.FileUsage()))
		}
	}
	// Interface usage statistics
	var interfaces []string
	interfaceByUsage := stats.ifstat.ByUsage()
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
		interfaces = append(interfaces, fmt.Sprintf("%10s r:%8s t:%8s", name, rx, tx))
	}
	for _, iface := range interfaceByUsage {
		if iface.TXBandwidthUsage() > 75.0 {
			stats.osind.Problems = append(stats.osind.Problems,
				fmt.Sprintf("TX bandwidth usage on (%v): %3.1f%%",
					iface.Name, iface.TXBandwidthUsage()))
		}
		if iface.RXBandwidthUsage() > 75.0 {
			stats.osind.Problems = append(stats.osind.Problems,
				fmt.Sprintf("RX bandwidth usage on (%v): %3.1f%%",
					iface.Name, iface.RXBandwidthUsage()))
		}
	}
	displayList(batchmode, "interface", layout, interfaces)
	// CPU stats by cgroup
	// TODO(syamp): should be sorted by quota usage
	var cgcpu, keys []string
	for name := range stats.cgCPU.Cgroups {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		v, ok := stats.cgCPU.Cgroups[name]
		if ok {
			name, _ = filepath.Rel(stats.cgCPU.Mountpoint, name)
			cpuUsagePct := (v.Usage() / stats.osind.CPUStat.Total()) * 100
			cpuQuotaPct := (v.Usage() / v.Quota()) * 100
			cpuThrottle := v.Throttle() * 100
			cgcpu = append(cgcpu, fmt.Sprintf("%20s %5s %6s %5s",
				truncate(name, 20),
				fmt.Sprintf("%3.1f%%", cpuUsagePct),
				fmt.Sprintf("q:%3.1f", v.Quota()),
				fmt.Sprintf("qpct:%3.1f%%", cpuQuotaPct)))
			if cpuThrottle > 0.1 {
				stats.osind.Problems =
					append(stats.osind.Problems, fmt.Sprintf(
						"CPU throttling on cgroup(%s): %3.1f%%",
						name, cpuThrottle))
			}
		}
	}
	displayList(batchmode, "cpu(cgroup)", layout, cgcpu)

	// Memory stats by cgroup
	// TODO(syamp): should be sorted by usage
	var cgmem []string
	keys = keys[:0]
	for name := range stats.cgMem.Cgroups {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		v, ok := stats.cgMem.Cgroups[name]
		if ok {
			name, _ = filepath.Rel(stats.cgMem.Mountpoint, name)
			memUsagePct := (v.Usage() / stats.osind.MemStat.Total()) * 100
			memQuota := v.SoftLimit()
			if memQuota > stats.osind.MemStat.Total() {
				memQuota = stats.osind.MemStat.Total()
			}
			memQuotaPct := (v.Usage() / v.SoftLimit()) * 100
			cgmem = append(cgmem, fmt.Sprintf("%20s %5s %10s %5s",
				truncate(name, 20),
				fmt.Sprintf("%3.1f%%", memUsagePct),
				fmt.Sprintf("q:%8s", misc.ByteSize(memQuota)),
				fmt.Sprintf("qpct:%3.1f%%", memQuotaPct)))
			if memQuotaPct > 75 {
				stats.osind.Problems =
					append(stats.osind.Problems, fmt.Sprintf(
						"Memory close to quota on cgroup(%s): %3.1f%%",
						name, memQuotaPct))
			}
		}
	}
	displayList(batchmode, "memory(cgroup)", layout, cgmem)
}
