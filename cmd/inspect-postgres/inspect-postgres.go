//Copyright (c) 2014 Square, Inc
//Launches metrics collector for postgres databases

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/postgres/stat"
)

func main() {
	var user, address, conf string
	var stepSec int
	var servermode, human, loop bool

	m := metrics.NewMetricContext("system")

	flag.StringVar(&user, "u", "postgres", "user using database")
	flag.BoolVar(&servermode, "server", false, "Runs continuously and exposes metrics as JSON on HTTP")
	flag.StringVar(&address, "address", ":12345", "address to listen on for http if running in server mode")
	flag.IntVar(&stepSec, "step", 2, "metrics are collected every step seconds")
	flag.StringVar(&conf, "conf", "/root/.my.cnf", "configuration file")
	flag.BoolVar(&human, "h", false, "Makes output in MB for human readable sizes")
	flag.BoolVar(&loop, "loop", false, "loop")
	flag.Parse()

	if servermode {
		go func() {
			http.HandleFunc("/metrics.json", m.HttpJsonHandler)
			log.Fatal(http.ListenAndServe(address, nil))
		}()
	}
	step := time.Millisecond * time.Duration(stepSec) * 1000

	sqlstat, err := stat.New(m, user, conf)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer sqlstat.Close()

	if loop {
		ticker := time.NewTicker(step * 2)
		for _ = range ticker.C {
			sqlstat.Collect()
			printAll(*sqlstat)
			//Print stats here, more stats than printed are actually collected/ stats con be removed from here
		}
	} else {
		sqlstat.Collect()
		printAll(*sqlstat)
	}

}

func printAll(sqlstat stat.PostgresStat) {
	fmt.Println("--------------------------")
	fmt.Println("Uptime: " + strconv.Itoa(int(sqlstat.Metrics.Uptime.Get())))
	fmt.Println("Version: " + strconv.FormatFloat(sqlstat.Metrics.Version.Get(), 'f', 5, 64))
	fmt.Println("TPS: " + strconv.Itoa(int(sqlstat.Metrics.TPS.Get())))
	fmt.Println("BlockReadsCache: " + strconv.FormatInt(int64(sqlstat.Metrics.BlockReadsCache.Get()), 10))
	fmt.Println("BlockReadsDisk: " + strconv.FormatInt(int64(sqlstat.Metrics.BlockReadsDisk.Get()), 10))
	fmt.Println("CacheHitPct: " + strconv.FormatFloat(sqlstat.Metrics.CacheHitPct.Get(), 'f', 5, 64))
	fmt.Println("CommitRatio: " + strconv.FormatFloat(sqlstat.Metrics.CommitRatio.Get(), 'f', 5, 64))
	fmt.Println("WalKeepSegments: " + strconv.FormatFloat(sqlstat.Metrics.WalKeepSegments.Get(), 'f', 5, 64))
	fmt.Println("SessionMax: " + strconv.FormatFloat(sqlstat.Metrics.SessionMax.Get(), 'f', 5, 64))
	fmt.Println("SessionCurrentTotal: " + strconv.FormatFloat(sqlstat.Metrics.SessionCurrentTotal.Get(), 'f', 5, 64))
	fmt.Println("SessionBusyPct: " + strconv.FormatFloat(sqlstat.Metrics.SessionBusyPct.Get(), 'f', 5, 64))
	fmt.Println("ConnMaxPct: " + strconv.FormatFloat(sqlstat.Metrics.ConnMaxPct.Get(), 'f', 5, 64))
	fmt.Println("OldestTrxs: " + strconv.FormatFloat(sqlstat.Metrics.OldestTrxS.Get(), 'f', 5, 64))
	fmt.Println("OldestQueryS: " + strconv.FormatFloat(sqlstat.Metrics.OldestQueryS.Get(), 'f', 5, 64))
	fmt.Println("ActiveLongRunQueries: " + strconv.FormatFloat(sqlstat.Metrics.ActiveLongRunQueries.Get(), 'f', 5, 64))
	fmt.Println("LockWaiters: " + strconv.FormatFloat(sqlstat.Metrics.LockWaiters.Get(), 'f', 5, 64))
	fmt.Println("Lock Counts: ")
	for mname, m := range sqlstat.Modes {
		locks := m.Locks.Get()
		fmt.Println("    " + mname + ": " + strconv.FormatFloat(locks, 'f', 2, 64))
	}
	fmt.Println("VacuumsAuto: " + strconv.FormatFloat(sqlstat.Metrics.VacuumsAutoRunning.Get(), 'f', 5, 64))
	fmt.Println("VacuumsManual: " + strconv.FormatFloat(sqlstat.Metrics.VacuumsManualRunning.Get(), 'f', 5, 64))
	fmt.Println("cpu: " + strconv.FormatFloat(sqlstat.Metrics.CpuPct.Get(), 'f', 5, 64))
	fmt.Println("mem: " + strconv.FormatFloat(sqlstat.Metrics.MemPct.Get(), 'f', 5, 64))
	fmt.Println("vsz: " + strconv.FormatFloat(sqlstat.Metrics.VSZ.Get(), 'f', 5, 64))
	fmt.Println("rss: " + strconv.FormatFloat(sqlstat.Metrics.RSS.Get(), 'f', 5, 64))
	fmt.Println("BinlogFiles: " + strconv.FormatFloat(sqlstat.Metrics.BinlogFiles.Get(), 'f', 5, 64))
	fmt.Println("DBSizeBinlogs: " + strconv.FormatFloat(sqlstat.Metrics.DBSizeBinlogs.Get(), 'f', 5, 64))
	for dbname, db := range sqlstat.DBs {
		size := db.SizeBytes.Get()
		units := " B"
		if true {
			size /= (1024 * 1024)
			units = " GB"
		}
		fmt.Println("    " + dbname + ": " + strconv.FormatFloat(size, 'f', 2, 64) + units)
		for tblname, tbl := range sqlstat.DBs[dbname].Tables {
			size := tbl.SizeBytes.Get()
			units := " B"
			if true {
				size /= (1024 * 1024)
				units = " GB"
			}
			fmt.Println("        " + tblname + ": " + strconv.FormatFloat(size, 'f', 2, 64) + units)
		}
	}
	fmt.Println("Seconds Behind Master: " + strconv.FormatFloat(sqlstat.Metrics.SecondsBehindMaster.Get(), 'f', 5, 64))
	fmt.Println("Slaves Connected To me: " + strconv.FormatFloat(sqlstat.Metrics.SlavesConnectedToMe.Get(), 'f', 5, 64))
	fmt.Println("SlaveBytesBehindMe: " + strconv.FormatFloat(sqlstat.Metrics.SlaveBytesBehindMe.Get(), 'f', 5, 64))
	fmt.Println("BackupsRunning: " + strconv.FormatFloat(sqlstat.Metrics.BackupsRunning.Get(), 'f', 5, 64))
	fmt.Println("UnsecureUsers: " + strconv.FormatFloat(sqlstat.Metrics.UnsecureUsers.Get(), 'f', 5, 64))
}
