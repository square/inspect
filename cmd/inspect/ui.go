// Copyright (c) 2014 Square, Inc
// +build linux darwin

package main

import (
	"github.com/gizak/termui"
	"github.com/square/inspect/cmd/inspect/osmain"
)

func uiWidgets() *osmain.DisplayWidgets {
	widgets := new(osmain.DisplayWidgets)
	widgets.Summary = termui.NewPar("Gathering statistics ...")
	widgets.ProcessesByCPU = termui.NewList()
	widgets.ProcessesByCPU.Border.Label = "CPU(c)"
	widgets.ProcessesByMemory = termui.NewList()
	widgets.ProcessesByMemory.Border.Label = "Memory(m)"
	widgets.ProcessesByIO = termui.NewList()
	widgets.ProcessesByIO.Border.Label = "IO(i)"
	widgets.DiskIOUsage = termui.NewList()
	widgets.DiskIOUsage.Border.Label = "Disk IO usage(d)"
	widgets.FileSystemUsage = termui.NewList()
	widgets.FileSystemUsage.Border.Label = "Filesystem usage(f)"
	widgets.InterfaceUsage = termui.NewList()
	widgets.InterfaceUsage.Border.Label = "Network usage(n)"
	widgets.CgroupsCPU = termui.NewList()
	widgets.CgroupsCPU.Border.Label = "CPU(cgroups)(C)"
	widgets.CgroupsMem = termui.NewList()
	widgets.CgroupsMem.Border.Label = "Memory(cgroups)(M)"
	widgets.Problems = termui.NewList()
	widgets.Problems.Border.Label = "Problems(p)"
	uiResetAttributes(widgets)
	return widgets
}

func uiResetAttributes(widgets *osmain.DisplayWidgets) {
	widgets.Summary.Height = 3
	widgets.ProcessesByCPU.Height = 5
	widgets.ProcessesByCPU.Border.Label = "CPU(c)"
	widgets.ProcessesByMemory.Height = 5
	widgets.ProcessesByMemory.Border.Label = "Memory(m)"
	widgets.ProcessesByIO.Height = 5
	widgets.ProcessesByIO.Border.Label = "IO(i)"
	widgets.DiskIOUsage.Height = 5
	widgets.DiskIOUsage.Border.Label = "Disk IO usage(d)"
	widgets.FileSystemUsage.Height = 5
	widgets.FileSystemUsage.Border.Label = "Filesystem usage(f)"
	widgets.InterfaceUsage.Height = 5
	widgets.InterfaceUsage.Border.Label = "Network usage(n)"
	widgets.CgroupsCPU.Height = 10
	widgets.CgroupsCPU.Border.Label = "CPU(cgroups)(C)"
	widgets.CgroupsMem.Height = 10
	widgets.CgroupsMem.Border.Label = "Memory(cgroups)(M)"
	widgets.Problems.Height = 10
	widgets.Problems.Border.Label = "Problems(p)"
}

func uiSummary(widgets *osmain.DisplayWidgets) *termui.Grid {
	body := termui.NewGrid()
	help := termui.NewPar("q:quit h:help (?):details")
	help.Height = 3
	body.AddRows(
		termui.NewRow(
			termui.NewCol(8, 0, widgets.Summary),
			termui.NewCol(4, 0, help)),
		termui.NewRow(
			termui.NewCol(6, 0, widgets.ProcessesByCPU),
			termui.NewCol(6, 0, widgets.ProcessesByMemory)),
		termui.NewRow(
			termui.NewCol(8, 0, widgets.ProcessesByIO),
			termui.NewCol(4, 0, widgets.DiskIOUsage)),
		termui.NewRow(
			termui.NewCol(6, 0, widgets.FileSystemUsage),
			termui.NewCol(6, 0, widgets.InterfaceUsage)),
		termui.NewRow(
			termui.NewCol(6, 0, widgets.CgroupsCPU),
			termui.NewCol(6, 0, widgets.CgroupsMem)),
		termui.NewRow(termui.NewCol(12, 0, widgets.Problems)))
	return body
}

func uiHelp() *termui.Grid {
	body := termui.NewGrid()
	kbHelp := termui.NewList()
	kbHelp.Items = []string{
		"h: Help",
		"s: Summary view",
		"(?): Details for subsection indicated by key in parentheses",
		"q: Quit",
	}
	kbHelp.Border.Label = "Keyboard shortcuts"
	kbHelp.Height = 20
	body.AddRows(termui.NewRow(termui.NewCol(12, 0, kbHelp)))
	return body
}

func uiDetail(detail *termui.List) *termui.Grid {
	body := termui.NewGrid()
	help := termui.NewPar("s:summary(main view) q:quit h:help")
	help.Height = 3
	help.Border.Label = "Keyboard shortcuts"
	detail.Height = 15
	body.AddRows(
		termui.NewRow(termui.NewCol(12, 0, help)),
		termui.NewRow(termui.NewCol(12, 0, detail)))
	return body
}

func uiRefresh() {
	termui.Body.Width = termui.TermWidth()
	termui.Body.Align()
	termui.Render(termui.Body)
}
