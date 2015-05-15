#### os

os is a collection of libraries for gathering
operating system metrics.

Supported platforms: linux, MacOSX 10.9

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


   * Per Process metrics
     * Platforms: Linux, MacOSX
     * On Linux
       * CPU
       * Memory
       * IO (requires root)
     * On MacOSX (requires root)
       * CPU
       * Memory
  

###### Installation

1. Get go
2. go get -v -u github.com/square/inspect/os

###### Documentation (WIP)

http://godoc.org/github.com/square/inspect/os

###### Example API use 


```go
// collect CPU stats
import "github.com/square/inspect/os"
import "github.com/square/inspect/metrics"

// Initialize a metric context
m := metrics.NewMetricContext("system")
	
// Collect CPU metrics every m.Step seconds
cstat := cpustat.New(m,  time.Millisecond*1000)

// Allow two samples to be collected. Since most metrics are counters.
time.Sleep(time.Millisecond * 1000 * 3)
fmt.Println(cstat.Usage())

```
###### Development
  * Designed to run as a long-lived process with minimal memory footprint - Re-use objects where possible.



###### TODO
  * TESTS
  * PerProcessStat on darwin doesn't include optimizations done for Linux. 
  * Add io metrics per process (need root priviliges)
