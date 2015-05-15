// Copyright (c) 2014 Square, Inc
// +build linux darwin

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/square/inspect/inspect/osmain"
	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/cpustat"
	"github.com/square/inspect/os/memstat"
	"github.com/square/inspect/os/misc"
	"github.com/square/inspect/os/pidstat"
	"github.com/mgutz/ansi"
)

// Number of Pids to display for top-N metrics
const DisplayPidCount = 3

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func main() {
	// options
	var batchmode, servermode bool
	var address string
	var stepSec int
	var nIter int

	flag.BoolVar(&batchmode, "b", false, "Run in batch mode; suitable for parsing")
	flag.BoolVar(&batchmode, "batchmode", false, "Run in batch mode; suitable for parsing")
	flag.IntVar(&nIter, "n", 0, "Quit after these many iterations")
	flag.IntVar(&nIter, "iterations", 0, "Quit after these many iterations")
	flag.BoolVar(&servermode, "server", false,
		"Runs continously and exposes metrics as JSON on HTTP")
	flag.StringVar(&address, "address", ":19999",
		"address to listen on for http if running in server mode")
	flag.IntVar(&stepSec, "step", 2,
		"metrics are collected every step seconds")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Options \n")
		fmt.Fprintf(os.Stderr, "------- \n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "Notes \n")
		fmt.Fprintf(os.Stderr, "------- \n")
		fmt.Fprintf(os.Stderr, "All CPU percentages are normalized to total number of logical cpus \n")
	}
	flag.Parse()

	if servermode {
		batchmode = true
	}

	if !batchmode {
		fmt.Println("Gathering statistics......")
	}

	// Initialize a metric context
	m := metrics.NewMetricContext("system")

	// Default step for collectors
	step := time.Millisecond * time.Duration(stepSec) * 1000

	// Collect cpu/memory/disk/per-pid metrics
	cstat := cpustat.New(m, step)
	mstat := memstat.New(m, step)
	procs := pidstat.NewProcessStat(m, step)

	// Filter processes which have < 1% of a CPU or < 1% memory
	procs.SetPidFilter(pidstat.PidFilterFunc(func(p *pidstat.PerProcessStat) bool {
		memUsagePct := (p.MemUsage() / mstat.Total()) * 100.0
		if p.CPUUsage() > 0.01 || memUsagePct > 1 {
			return true
		}
		return false
	}))

	// pass the collected metrics to OS dependent set if they
	// need it
	osind := new(osmain.OsIndependentStats)
	osind.Cstat = cstat
	osind.Mstat = mstat
	osind.Procs = procs

	// register os dependent metrics
	// these could be specific to the OS (say cgroups)
	// or stats which are implemented not on all supported
	// platforms yet
	d := osmain.RegisterOsDependent(m, step, osind)

	// run http server
	if servermode {
		go func() {
			http.HandleFunc("/api/v1/metrics.json", m.HttpJsonHandler)
			log.Fatal(http.ListenAndServe(address, nil))
		}()
	}

	// command line refresh every 2 step
	ticker := time.NewTicker(step * 2)
	iterationsRun := 0
	for _ = range ticker.C {

		// Problems
		var problems []string

		// Quit after n iterations if specified
		iterationsRun++
		if nIter > 0 && iterationsRun > nIter {
			break
		}

		if !batchmode {
			fmt.Printf("\033[2J") // clear screen
			fmt.Printf("\033[H")  // move cursor top left top
		}

		memPctUsage := (mstat.Usage() / mstat.Total()) * 100
		cpuPctUsage := (cstat.Usage() / cstat.Total()) * 100
		cpuUserspacePctUsage := (cstat.UserSpace() / cstat.Total()) * 100
		cpuKernelPctUsage := (cstat.Kernel() / cstat.Total()) * 100
		fmt.Println(fmt.Sprintf("total: cpu: %3.1f%% user: %3.1f%%, kernel: %3.1f%%, mem: %3.1f%%",
			cpuPctUsage, cpuUserspacePctUsage, cpuKernelPctUsage, memPctUsage))

		if cpuPctUsage > 80.0 {
			problems = append(problems, "CPU usage > 80%")
		}
		if cpuKernelPctUsage > 30.0 {
			problems = append(problems, "CPU usage in kernel > 30%")
		}
		if memPctUsage > 80.0 {
			problems = append(problems, "Memory usage > 80%")
		}

		// Top processes by usage
		procsByCPUUsage := procs.ByCPUUsage()
		procsByMemUsage := procs.ByMemUsage()
		n := DisplayPidCount
		if len(procsByCPUUsage) < n {
			n = len(procsByCPUUsage)
		}
		if len(procsByMemUsage) < n {
			n = len(procsByMemUsage)
		}
		// reverse colors
		fmt.Println("\033[7m")
		fmt.Println(fmt.Sprintf("%8s %10s %10s %8s | %8s %10s %10s %8s",
			"CPU",
			"COMMAND",
			"USER",
			"PID",
			"MEM",
			"COMMAND",
			"USER",
			"PID"), ansi.ColorCode("reset"))
		for i := 0; i < n; i++ {
			cpuUsagePct := (procsByCPUUsage[i].CPUUsage() / cstat.Total()) * 100
			fmt.Println(fmt.Sprintf("%8s %10s %10s %8s | %8s %10s %10s %8s",
				fmt.Sprintf("%3.1f%%", cpuUsagePct),
				truncate(procsByCPUUsage[i].Comm(), 10),
				truncate(procsByCPUUsage[i].User(), 10),
				procsByCPUUsage[i].Pid(),
				misc.ByteSize(procsByMemUsage[i].MemUsage()),
				truncate(procsByMemUsage[i].Comm(), 10),
				truncate(procsByMemUsage[i].User(), 10),
				procsByMemUsage[i].Pid()))
		}
		for i := n; i < DisplayPidCount; i++ {
			fmt.Println(fmt.Sprintf("%8s %10s %10s %8s | %8s %10s %10s %8s",
				"-", "-", "-", "-", "-", "-", "-", "-"))
		}

		osmain.PrintOsDependent(d, batchmode)

		for i := range problems {
			msg := problems[i]
			if !batchmode {
				msg = ansi.Color(msg, "red")
			}
			fmt.Println("Problem: ", msg)
		}

		// be aggressive about reclaiming memory
		// tradeoff with CPU usage
		runtime.GC()
		debug.FreeOSMemory()
	}
}
