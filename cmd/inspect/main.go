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

	"github.com/gizak/termui"
	"github.com/sorawee/inspect/cmd/inspect/osmain"
	"github.com/sorawee/inspect/metrics"
)

var layout *osmain.DisplayWidgets
var evt <-chan termui.Event

func setupDisplay() *osmain.DisplayWidgets {
	err := termui.Init()
	termui.UseTheme("helloworld")
	if err != nil {
		log.Fatalf("Unable to initialize termui", err) // exits with 1
	}
	layout := new(osmain.DisplayWidgets)
	layout.Summary = termui.NewPar("Gathering statistics ...")
	layout.Summary.Height = 3
	help := termui.NewPar("Press q to quit")
	help.Height = 3
	layout.ProcessesByCPU = termui.NewList()
	layout.ProcessesByCPU.Height = 5
	layout.ProcessesByCPU.Border.Label = "CPU"
	layout.ProcessesByMemory = termui.NewList()
	layout.ProcessesByMemory.Height = 5
	layout.ProcessesByMemory.Border.Label = "Memory"
	layout.ProcessesByIO = termui.NewList()
	layout.ProcessesByIO.Height = 5
	layout.ProcessesByIO.Border.Label = "IO"
	layout.DiskIOUsage = termui.NewList()
	layout.DiskIOUsage.Height = 5
	layout.DiskIOUsage.Border.Label = "Disk IO usage"
	layout.FileSystemUsage = termui.NewList()
	layout.FileSystemUsage.Height = 5
	layout.FileSystemUsage.Border.Label = "Filesystem usage"
	layout.InterfaceUsage = termui.NewList()
	layout.InterfaceUsage.Height = 5
	layout.InterfaceUsage.Border.Label = "Network usage"
	layout.CgroupsCPU = termui.NewList()
	layout.CgroupsCPU.Height = 10
	layout.CgroupsCPU.Border.Label = "CPU(cgroups)"
	layout.CgroupsMem = termui.NewList()
	layout.CgroupsMem.Height = 10
	layout.CgroupsMem.Border.Label = "Memory(cgroups)"
	layout.Problems = termui.NewList()
	layout.Problems.Height = 10
	layout.Problems.Border.Label = "Problems"
	termui.Body.AddRows(
		termui.NewRow(
			termui.NewCol(8, 0, layout.Summary),
			termui.NewCol(4, 0, help)),
		termui.NewRow(
			termui.NewCol(6, 0, layout.ProcessesByCPU),
			termui.NewCol(6, 0, layout.ProcessesByMemory)),
		termui.NewRow(
			termui.NewCol(8, 0, layout.ProcessesByIO),
			termui.NewCol(4, 0, layout.DiskIOUsage)),
		termui.NewRow(
			termui.NewCol(6, 0, layout.FileSystemUsage),
			termui.NewCol(6, 0, layout.InterfaceUsage)),
		termui.NewRow(
			termui.NewCol(6, 0, layout.CgroupsCPU),
			termui.NewCol(6, 0, layout.CgroupsMem)),
		termui.NewRow(termui.NewCol(12, 0, layout.Problems)))
	termui.Body.Width = termui.TermWidth()
	termui.Body.Align()
	termui.Render(termui.Body)
	return layout
}

func refreshUI() {
	termui.Body.Width = termui.TermWidth()
	termui.Body.Align()
	termui.Render(termui.Body)
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
		layout = setupDisplay()
		evt = termui.EventCh()
	}
	// Initialize a metric context
	m := metrics.NewMetricContext("system")
	// Default step for collectors
	step := time.Millisecond * time.Duration(stepSec) * 1000
	// Register various stats we are interested in tracking
	stats := osmain.Register(m, step)
	// run http server
	if servermode {
		go func() {
			http.HandleFunc("/api/v1/metrics.json", m.HttpJsonHandler)
			log.Fatal(http.ListenAndServe(address, nil))
		}()
	}

	iterationsRun := 0
	for {
		if !batchmode {
			// handle keypresses in interactive mode
			select {
			case e := <-evt:
				if e.Type == termui.EventKey && e.Ch == 'q' {
					termui.Close()
					return
				}
				if e.Type == termui.EventResize {
					refreshUI()
				}
			default:
				break // breaks out of select
			}
		}
		// Clear previous problems
		var problems []string
		stats.Problems = problems
		// Quit after n iterations if specified
		iterationsRun++
		if nIter > 0 && iterationsRun > nIter {
			break
		}
		stats.Print(batchmode, layout)
		if !batchmode {
			termui.Render(termui.Body)
		}
		// sleep for step
		time.Sleep(step / 2)
		// be aggressive about reclaiming memory
		// tradeoff with CPU usage
		runtime.GC()
		debug.FreeOSMemory()
	}
}
