Metrics is a simple library for collecting metrics for golang applications. 

###### Example API use

```go
// Initialize a metric context
m := metrics.NewMetricContext("system")

// Create a basic counter, all ops are atomic
c := metrics.NewBasicCounter()

c.Add(n)    // increment counter by delta n
c.Set(n)    // Set counter value to n


// Create a new counter; has additional state associated with it
// to calculate rate

c := metrics.NewCounter()

c.Add(n)    // increment counter by delta n
c.Set(n)    // Set counter value to n

r := c.ComputeRate() // compute rate of change/sec

// Create a new gauge
// Set/Get acquire a mutex
c := metrics.NewGauge()
c.Set(12.0) // Set Value
c.Get() // get Value

// StatsTimer - useful for computing statistics on timed operations
s := metrics.NewStatsTimer()

t := s.Start() // returns a timer
s.Stop(t) // stop the timer

// Example
func (* Webapp) ServeRequest(uri string) error {
	t := s.Start()

	// do something
	s.Stop(t)
}
pctile_75th, err := s.Percentile(75)
if err == nil {
	fmt.Println("Percentile latency for 75 pctile: ", pctile_75th)
}


// Launch a goroutine to serve metrics via http json
go func() {
	http.HandleFunc("/metrics.json", m.HttpJsonHandler)
	http.ListenAndServe("localhost:12345", nil)
}

// Get metrics via http json.
resp, err := http.Get("http://localhost:12345/metrics.json")
```

##### Dependencies
Package dependency is managed by godep (https://github.com/tools/godep). Follow the docs there when adding/removing/updating
package dependencies.
