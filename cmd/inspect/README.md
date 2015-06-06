#### inspect

inspect command line is a utility that gives a
brief overview on current state of system resource
usage and ability to dig through details. 

Supported platforms: linux, MacOSX 10.9

inspect aims to evolve to be an intelligent tool that
can spot problems.

Currently it can spot few easy ones:
  * process X is throttled on CPU because of cgroup restrictions
  * System wide resource usage problems (disk/cpu/mem/net)


###### Installation

1. Get go
2. go get -v -u github.com/square/inspect/inspect # fetches packages and builds

###### Dependencies
Package dependency is managed by godep (https://github.com/tools/godep). Follow the docs there when adding/removing/updating
package dependencies.

###### Documentation
http://godoc.org/github.com/square/inspect/os

http://godoc.org/github.com/square/inspect/metrics

###### Usage
###### Command line

./bin/inspect (run as root for IO statistics on Linux)

##### Summary View

<img src="https://raw.githubusercontent.com/square/inspect/master/cmd/inspect/screenshots/summary.png" width="540" height="507">

##### Details View
<img src="https://raw.githubusercontent.com/square/inspect/master/cmd/inspect/screenshots/details.png" width="403" height="190">

###### Server 

*inspect* can be run in server mode to run continously and expose metrics via HTTP JSON api

./bin/inspect  -server -address :12345

```
s@c62% curl localhost:12345/api/v1/metrics.json 2>/dev/null
[
{"type": "gauge", "name": "memstat.Mapped", "value": 16314368.000000},
{"type": "gauge", "name": "memstat.HugePages_Rsvd", "value": 0.000000},
{"type": "gauge", "name": "diskstat.sr0.IOInProgress", "value": 0.000000},
{"type": "gauge", "name": "memstat.cgroup.small.Inactive_anon", "value": 0.000000},
....... truncated
{"type": "counter", "name": "diskstat.sdb.ReadSectors", "value": 7288530, "rate": 0.000000},
{"type": "counter", "name": "interfacestat.eth0.TXpackets", "value": 6445308, "rate": 4.333320},
{"type": "counter", "name": "interfacestat.eth0.TXframe", "value": 0, "rate": 0.000000},
{"type": "counter", "name": "pidstat.pid1.Utime", "value": 31, "rate": 0.000000},
{"type": "counter", "name": "pidstat.pid29769.Utime", "value": 74296, "rate": 0.000000}]
```

###### Todo
  * Refactor to make inspect command line work with any application type
  * TESTS
  * Rules for inspection need to seperated out into user supplied code/config. Currently inspect command line has hard-coded guesswork
  * PerProcessStat on darwin doesn't include optimizations done for Linux. 
  * Command line utility needs much nicer formatting and options to dig into per process/cgroup details
  * Add caching support to reduce load when multiple invocations of inspect happen.
  * API to collect and expose historical/current statistics
