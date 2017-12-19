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
	"sync"
	"time"

	"github.com/gizak/termui"

	"github.com/square/inspect/cmd/inspect/osmain"
	"github.com/square/inspect/metrics"
)

func main() {
	// never use more than one process
	runtime.GOMAXPROCS(1)
	// wait group to let the main program run forever in batchmode
	var wg sync.WaitGroup
	// options
	var batchmode, servermode bool
	var address string
	var stepSec int
	var nIter int
	var evt <-chan termui.Event
	var widgets *osmain.DisplayWidgets
	var uiSummaryBody *termui.Grid
	var uiHelpBody *termui.Grid
	var uiDetailList *termui.List

	flag.BoolVar(&batchmode, "b", false, "Run in batch mode; suitable for parsing")
	flag.BoolVar(&batchmode, "batchmode", false, "Run in batch mode; suitable for parsing")
	flag.IntVar(&nIter, "n", 0, "Quit after these many iterations")
	flag.IntVar(&nIter, "iterations", 0, "Quit after these many iterations")
	flag.BoolVar(&servermode, "server", false,
		"Runs continuously and exposes metrics as JSON on HTTP")
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
		err := termui.Init()
		termui.UseTheme("helloworld")
		if err != nil {
			log.Fatal("Unable to initialize termui:", err)
		}
		widgets = uiWidgets()
		uiSummaryBody = uiSummary(widgets)
		uiHelpBody = uiHelp()
		uiDetailList = termui.NewList()
		evt = termui.EventCh()
		// display summary view
		termui.Body = uiSummaryBody
		uiRefresh()
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
			http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {})
			http.HandleFunc("/api/v1/metrics.json", m.HttpJsonHandler)
			log.Fatal(http.ListenAndServe(address, nil))
		}()
	}

	iterationsRun := 0
	// runs forever
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			// Clear previous problems
			var problems []string
			stats.Problems = problems
			// Quit after n iterations if specified
			iterationsRun++
			if nIter > 0 && iterationsRun > nIter {
				break
			}
			stats.Print(batchmode, widgets)
			if !batchmode {
				termui.Render(termui.Body)
			}
			time.Sleep(step)
			// be aggressive about reclaiming memory
			// tradeoff with CPU usage
			runtime.GC()
			debug.FreeOSMemory()
		}
	}()

	if !batchmode {
		for {
			e := <-evt
			if e.Type == termui.EventKey {
				switch e.Ch {
				case 'q':
					termui.Close()
					return
				case 'c':
					uiDetailList = widgets.ProcessesByCPU
					termui.Body = uiDetail(uiDetailList)
				case 'd':
					uiDetailList = widgets.DiskIOUsage
					termui.Body = uiDetail(uiDetailList)
				case 'C':
					uiDetailList = widgets.CgroupsCPU
					termui.Body = uiDetail(uiDetailList)
				case 'f':
					uiDetailList = widgets.FileSystemUsage
					termui.Body = uiDetail(uiDetailList)
				case 'm':
					uiDetailList = widgets.ProcessesByMemory
					termui.Body = uiDetail(uiDetailList)
				case 'p':
					uiDetailList = widgets.Problems
					termui.Body = uiDetail(uiDetailList)
				case 'M':
					uiDetailList = widgets.CgroupsMem
					termui.Body = uiDetail(uiDetailList)
				case 'n':
					uiDetailList = widgets.InterfaceUsage
					termui.Body = uiDetail(uiDetailList)
				case 'i':
					uiDetailList = widgets.ProcessesByIO
					termui.Body = uiDetail(uiDetailList)
				case 's':
					uiResetAttributes(widgets)
					termui.Body = uiSummaryBody
				case 'h':
					termui.Body = uiHelpBody
				}
				uiRefresh()
			}
			if e.Type == termui.EventResize {
				uiRefresh()
			}
		}
	}
	wg.Wait()
}
