#### os
os is a collection of libraries for gathering operating system metrics.

######
Implemented libraries:
   * CPU usage 
      * Platforms: Linux, MacOSX
      * For linux, per cgroup information is included
   * Memory usage
      * Platforms: Linux, MacOSX
      * For linux, per cgroup information is included
   * Filesystem usage
      * Platforms: Linux
   * Interface usage
      * Platforms: Linux
   * IO subsystem usage
      * Platforms: Linux
   * TCP
      * Platforms: Linux
   * Per Process metrics
     * Platforms: Linux, MacOSX
     * On Linux
       * CPU
       * Memory
       * IO (requires root)
     * On MacOSX (requires root)
       * CPU
       * Memory
   * Load average
    * Platforms: Linux
  
###### Installation
1. Get go
2. go get -v -u github.com/square/inspect/os/[module]

###### Documentation
http://godoc.org/github.com/square/inspect/os/cpustat

http://godoc.org/github.com/square/inspect/os/memstat

http://godoc.org/github.com/square/inspect/os/pidstat

http://godoc.org/github.com/square/inspect/os/tcpstat

http://godoc.org/github.com/square/inspect/os/fsstat

http://godoc.org/github.com/square/inspect/os/interfacestat

http://godoc.org/github.com/square/inspect/os/loadstat

###### Example API use 
```go
package main

import (
	"fmt"
	"time"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/cpustat"
)

func main() {
	// Initialize a metric context
	m := metrics.NewMetricContext("system")

	// Collect CPU metrics every 1 (Step) second
	cstat := cpustat.New(m, time.Millisecond*1000)

	// Allow two samples to be collected. Since most metrics are counters.
	time.Sleep(time.Millisecond * 1000 * 3)
	fmt.Println("CPU usage (%) : ", (cstat.Usage()/cstat.Total())*100)
}
```
###### Development
  * Designed to run as a long-lived process with minimal memory footprint - Re-use objects where possible.
  * Please run golint.

###### TODO
  * TESTS
  * PerProcessStat on darwin doesn't include optimizations done for Linux. 
  * Add io metrics per process (need root priviliges)
