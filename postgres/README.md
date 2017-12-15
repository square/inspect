**postgres** is a collection of libraries for gathering metrics of postgres databases.
**postgres** libraries can gather the following metrics:
- Version number
- Slave Stats
- Global Stats
- Binlog Stats
- Stacked Query info
- Session Info
- Innodb stats
- Long Run Query info
- Query Response Times

#### Installation
1. `go get -v -u github.com/square/inspect/postgres`

#### Example API Use

```go
// Import packages
import "github.com/square/inspect/postgres"
import "github.com/square/inspect/metrics"

// Initialize a metric context
m := metrics.NewMetricContext("postgres")

// Collect postgres metrics every m.Step seconds
sqlstats := postgresstat.New(m, time.Millisecond*2000)

// Collects postgres metrics for specific databases and tables
sqltablestats := postgresstattable.New(m, time.Millisecond*2000)
```

All metrics collected are exported, so any metric may be accessed using Get():
```
// Print the number of queries accessed
fmt.Println(sqlstats.Metrics.Queries.Get())

// Print the size of table t1 in database db1
fmt.Println(sqltablestats.DBs["db1"].Tables["t1"].Metrics.SizeBytes.Get())
```
