//Copyright (c) 2014 Square, Inc

//Launches metrics collector for mysql databases
//

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"code.google.com/p/goconf/conf"
	"github.com/square/inspect/metricchecks"
	"github.com/square/inspect/metrics"
	"github.com/square/inspect/mysql/dbstat"
	"github.com/square/inspect/mysql/tablestat"
	"github.com/square/inspect/mysql/userstat"
)

func main() {
	var user, password, host, address, cnf, form, checkConfigFile string
	var stepSec int
	var servermode, human, loop bool
	var checkConfig *conf.ConfigFile

	m := metrics.NewMetricContext("system")

	flag.StringVar(&user, "u", "root", "user using database")
	flag.StringVar(&password, "p", "", "password for database")
	flag.StringVar(&host, "h", "",
		"address and protocol of the database to connect to. leave blank for tcp(127.0.0.1:3306)")
	flag.BoolVar(&servermode, "server", false,
		"Runs continously and exposes metrics as JSON on HTTP")
	flag.StringVar(&address, "address", ":12345",
		"address to listen on for http if running in server mode")
	flag.IntVar(&stepSec, "step", 2, "metrics are collected every step seconds")
	flag.StringVar(&cnf, "cnf", "/root/.my.cnf", "configuration file")
	flag.StringVar(&form, "form", "graphite", "output format of metrics to stdout")
	flag.BoolVar(&human, "human", false,
		"Makes output in MB for human readable sizes")
	flag.BoolVar(&loop, "loop", false, "loop on collecting metrics")
	flag.StringVar(&checkConfigFile, "check", "", "config file to check metrics with")
	flag.Parse()

	if servermode {
		go func() {
			http.HandleFunc("/api/v1/metrics.json/", m.HttpJsonHandler)
			log.Fatal(http.ListenAndServe(address, nil))
		}()
	}
	step := time.Millisecond * time.Duration(stepSec) * 1000

	var err error
	var c metricchecks.Checker
	checkConfig = conf.NewConfigFile()
	if checkConfigFile != "" {
		cnf, err := metricchecks.FileToConfig(checkConfigFile)
		if err != nil {
			checkConfigFile = ""
		} else {
			checkConfig = cnf
		}
	}

	c, err = metricchecks.New("", checkConfig)
	if err != nil {
		checkConfigFile = ""
	}

	//initialize metrics collectors to not loop and collect
	sqlstatDBs, err := dbstat.New(m, user, password, host, cnf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sqlstatTables, err := tablestat.New(m, user, password, host, cnf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	sqlstatUsers, err := userstat.New(m, user, password, host, cnf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	sqlstatDBs.Collect()
	sqlstatTables.Collect()
	sqlstatUsers.Collect()

	if checkConfigFile != "" {
		checkMetrics(c, m)
	}
	outputTableMetrics(sqlstatDBs, sqlstatTables, m, form)
	outputUserMetrics(sqlstatUsers, m, form)
	if loop {
		ticker := time.NewTicker(step)
		for _ = range ticker.C {
			sqlstatDBs.Collect()
			sqlstatTables.Collect()
			sqlstatUsers.Collect()
			outputTableMetrics(sqlstatDBs, sqlstatTables, m, form)
			outputUserMetrics(sqlstatUsers, m, form)
		}
	}
	sqlstatDBs.Close()
	sqlstatTables.Close()
	sqlstatUsers.Close()
}

func checkMetrics(c metricchecks.Checker, m *metrics.MetricContext) error {
	err := c.NewScopeAndPackage()
	if err != nil {
		return err
	}
	err = c.InsertMetricValuesFromContext(m)
	if err != nil {
		return err
	}
	checks, err := c.CheckAll()
	for _, check := range checks {
		fmt.Println(check)
	}
	return err
}

//output metrics in specific output format
func outputTableMetrics(d *dbstat.MysqlStatDBs, t *tablestat.MysqlStatTables,
	m *metrics.MetricContext, form string) {
	//print out json packages
	if form == "json" {
		m.EncodeJSON(os.Stdout)
	}
	//print out in graphite form:
	//<metric_name> <metric_value>
	if form == "graphite" {
		d.FormatGraphite(os.Stdout)
		t.FormatGraphite(os.Stdout)
	}
}

//output metrics in specific output format
func outputUserMetrics(u *userstat.MysqlStatUsers,
	m *metrics.MetricContext, form string) {
	//print out json packages
	if form == "json" {
		m.EncodeJSON(os.Stdout)
	}
	//print out in graphite form:
	//<metric_name> <metric_value>
	if form == "graphite" {
		u.FormatGraphite(os.Stdout)
	}
}
