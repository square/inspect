#### mysql
**mysql** is a collection of libraries for gathering various metrics for MySQL servers

**mysql** libraries can gather the following metrics:
 * Version information
 * Slave Stats
 * Global Stats
 * Binlog Stats
 * Stacked Query info - Information about queries that are repeated and getting stacked.
 * Session Info
 * Innodb stats
 * Long Run Query info - Information about queries that are running for a long period of time.
 * Query Response Times

#### Installation
1. `go get -v -u github.com/square/inspect/mysql`

#### Usage

```go
// Import packages
import "github.com/square/inspect/mysql"
import "github.com/square/inspect/metrics"

// Initialize a metric context
m := metrics.NewMetricContext("mysql")

// Collect mysql metrics every m.Step seconds
// Username and password may be left as "" if a config file is specified
sqlstats := mysqlstat.New(m, <username>, <password>, <config file name>)

// Collects mysql metrics for specific databases and tables
// Username and password may be left as "" if a config file is specified
sqltablestats := mysqlstattable.New(m, <username>, <password>, <config file name>)

// Collect all metrics
sqlstats.Collect()

// Collect a group of metrics:
sqlstat.GetVersion()
sqlstat.GetSlaveStats()
sqlstat.GetGlobalStatus()
sqlstat.GetBinlogStats()
sqlstat.GetStackedQueries()
sqlstat.GetSessions()
sqlstat.GetNumLongRunQueries()
sqlstat.GetQueryResponseTime()
sqlstat.GetBackups()
sqlstat.GetOldestQuery()
sqlstat.GetOldestTrx()
sqlstat.GetBinlogFiles()
sqlstat.GetInnodbBufferpoolMutexWaits()
sqlstat.GetSecurity()
sqltablestats.GetDBSizes()
sqltablestats.GetTableSizes()
sqltablestats.GetTableStatistics()
```
All metrics collected are exported to metriccontext. 

```
// Print the number of queries accessed
fmt.Println(sqlstats.Metrics.Queries.Get())

// Print the number of select queries per second
fmt.Println(sqlstats.ComSelect.ComputeRate())

// Print the size of table t1 in databse db1
fmt.Println(sqltablestats.DBs["db1"].Tables["t1"].Metrics.SizeBytes.Get())
```

#### Grouping of metrics

```
  //GetSlave Stats
	SlaveSecondsBehindMaster *metrics.Gauge
	SlaveSeqFile             *metrics.Gauge
	SlavePosition            *metrics.Counter

	//GetGlobalStatus
	BinlogCacheDiskUse        *metrics.Counter
	BinlogCacheUse            *metrics.Counter
	ComAlterTable             *metrics.Counter
	ComBegin                  *metrics.Counter
	ComCommit                 *metrics.Counter
	ComCreateTable            *metrics.Counter
	ComDelete                 *metrics.Counter
	ComDeleteMulti            *metrics.Counter
	ComDropTable              *metrics.Counter
	ComInsert                 *metrics.Counter
	ComInsertSelect           *metrics.Counter
	ComReplace                *metrics.Counter
	ComReplaceSelect          *metrics.Counter
	ComRollback               *metrics.Counter
	ComSelect                 *metrics.Counter
	ComUpdate                 *metrics.Counter
	ComUpdateMulti            *metrics.Counter
	CreatedTmpDiskTables      *metrics.Counter
	CreatedTmpFiles           *metrics.Counter
	CreatedTmpTables          *metrics.Counter
	InnodbCurrentRowLocks     *metrics.Counter
	InnodbLogOsWaits          *metrics.Counter
	InnodbRowLockCurrentWaits *metrics.Counter
	InnodbRowLockTimeAvg      *metrics.Counter
	InnodbRowLockTimeMax      *metrics.Counter
	Queries                   *metrics.Counter
	SortMergePasses           *metrics.Counter
	ThreadsConnected          *metrics.Counter
	Uptime                    *metrics.Counter
	ThreadsRunning            *metrics.Counter

	//GetInnodbBufferPoolMutexWaits
	InnodbBufpoolLRUMutexOSWait *metrics.Counter
	InnodbBufpoolZipMutexOSWait *metrics.Counter

	//GetOldestQueryS
	OldestQueryS *metrics.Gauge

	//GetOldestTrxS
	OldestTrxS *metrics.Gauge

	//BinlogFiles
	BinlogFiles *metrics.Gauge
	BinlogSize  *metrics.Gauge

	//GetNumLongRunQueries
	ActiveLongRunQueries *metrics.Gauge

	//GetVersion
	Version *metrics.Gauge

	//GetBinlogStats
	BinlogSeqFile  *metrics.Gauge
	BinlogPosition *metrics.Counter

	//GetStackedQueries
	IdenticalQueriesStacked *metrics.Gauge
	IdenticalQueriesMaxAge  *metrics.Gauge

	//GetSessions
	ActiveSessions          *metrics.Gauge
	BusySessionPct          *metrics.Gauge
	CurrentSessions         *metrics.Gauge
	CurrentConnectionsPct   *metrics.Gauge
	LockedSessions          *metrics.Gauge
	MaxConnections          *metrics.Gauge
	SessionTablesLocks      *metrics.Gauge
	SessionGlobalReadLocks  *metrics.Gauge
	SessionsCopyingToTable  *metrics.Gauge
	SessionsStatistics      *metrics.Gauge
	UnauthenticatedSessions *metrics.Gauge

	//GetInnodbStats
	OSFileReads                   *metrics.Gauge
	OSFileWrites                  *metrics.Gauge
	AdaptiveHash                  *metrics.Gauge
	AvgBytesPerRead               *metrics.Gauge
	BufferPoolHitRate             *metrics.Gauge
	BufferPoolSize                *metrics.Gauge
	CacheHitPct                   *metrics.Gauge
	InnodbCheckpointAge           *metrics.Gauge
	InnodbCheckpointAgeTarget     *metrics.Gauge
	DatabasePages                 *metrics.Gauge
	DictionaryCache               *metrics.Gauge
	DictionaryMemoryAllocated     *metrics.Gauge
	FileSystem                    *metrics.Gauge
	FreeBuffers                   *metrics.Gauge
	FsyncsPerSec                  *metrics.Gauge
	InnodbHistoryLinkList         *metrics.Gauge
	InnodbLastCheckpointAt        *metrics.Gauge
	LockSystem                    *metrics.Gauge
	InnodbLogFlushedUpTo          *metrics.Gauge
	LogIOPerSec                   *metrics.Gauge
	InnodbLogSequenceNumber       *metrics.Gauge
	InnodbMaxCheckpointAge        *metrics.Gauge
	InnodbModifiedAge             *metrics.Gauge
	ModifiedDBPages               *metrics.Gauge
	OldDatabasePages              *metrics.Gauge
	PageHash                      *metrics.Gauge
	PagesFlushedUpTo              *metrics.Gauge
	PagesMadeYoung                *metrics.Gauge
	PagesRead                     *metrics.Gauge
	InnodbLogWriteRatio           *metrics.Gauge
	InnodbPendingCheckpointWrites *metrics.Gauge
	InnodbPendingLogWrites        *metrics.Gauge
	PendingReads                  *metrics.Gauge
	PendingWritesLRU              *metrics.Gauge
	ReadsPerSec                   *metrics.Gauge
	RecoverySystem                *metrics.Gauge
	TotalMem                      *metrics.Gauge
	TotalMemByReadViews           *metrics.Gauge
	TransactionID                 *metrics.Gauge
	InnodbTransactionsNotStarted  *metrics.Gauge
	InnodbUndo                    *metrics.Gauge
	WritesPerSec                  *metrics.Gauge

	//GetBackups
	BackupsRunning *metrics.Gauge

	//GetSecurity
	UnsecureUsers *metrics.Gauge

	//Query response time metrics
	QueryResponseSec_000001  *metrics.Counter
	QueryResponseSec_00001   *metrics.Counter
	QueryResponseSec_0001    *metrics.Counter
	QueryResponseSec_001     *metrics.Counter
	QueryResponseSec_01      *metrics.Counter
	QueryResponseSec_1       *metrics.Counter
	QueryResponseSec1_       *metrics.Counter
	QueryResponseSec10_      *metrics.Counter
	QueryResponseSec100_     *metrics.Counter
	QueryResponseSec1000_    *metrics.Counter
	QueryResponseSec10000_   *metrics.Counter
	QueryResponseSec100000_  *metrics.Counter
	QueryResponseSec1000000_ *metrics.Counter
```

##Testing 

Packages are tested using Go's testing package.

To test:
  * `go test -v ./...` For these tests, logging is redirected to a file `test.log` 

Tests for each metric may be added to `mysqlstat_test.go` and `mysqlstat-tables_test.go`. These tests do not connect to a database. Instead, the desired test input is hard coded into each test. Testing for the parser for the Innodb metrics are located in `mysqltools_test.go`. 

