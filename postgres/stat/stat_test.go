//Copyright (c) 2014 Square, Inc
//
// Tests the metrics collecting functions for postgresstat.go.
// Tests do not connect to a database, dummy functions are used instead and return hard coded
// input. Testing connections (will) be done in postgrestools_test.go.
//
// Each test first sets input data, and uses Collect() to gather
// metrics rather than calling that metric's get function.
// This ensures that other functions still work on malformed
// or missing input, such as what would happen with an incorrect query.
// Testing the correctness of postgres queries should be done manually.
//
// Integration/Acceptance testing is harder and is avoided because
// creating and populating a fake database with the necessary information
// may be more trouble than is worth. Manual testing may be required for
// full acceptance tests.
package stat

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/square/inspect/metrics"
)

type testPostgresDB struct {
	Logger *log.Logger
}

var (
	//testquerycol and testqueryrow map a query string to the desired test result
	//simulates QueryReturnColumnDict()
	testquerycol = map[string]map[string][]string{}

	//Simulates QueryMapFirstColumnToRow
	testqueryrow = map[string]map[string][]string{}

	//Mapping of metric and its expected value
	// defined as map of interface{}->interface{} so
	// can switch between metrics.Gauge and metrics.Counter
	// and between float64 and uint64 easily
	expectedValues = map[interface{}]interface{}{}

	//  expecting lots of log messages because of tests
	//  redirect to this file
	logFile, _ = os.OpenFile("./test.log", os.O_WRONLY|os.O_CREATE|os.O_SYNC, 0644)
)

//functions that behave like mysqltools but we can make it return whatever
func (s *testPostgresDB) QueryReturnColumnDict(query string) (map[string][]string, error) {
	return testquerycol[query], nil
}

func (s *testPostgresDB) QueryMapFirstColumnToRow(query string) (map[string][]string, error) {
	return testquerycol[query], nil
}

func (s *testPostgresDB) Log(in interface{}) {
	_, f, line, ok := runtime.Caller(1)
	if ok {
		s.Logger.Println("Log from: " + f + " line: " + strconv.Itoa(line))
	}
	s.Logger.Println(in)
}

func (s *testPostgresDB) Close() {
	return
}

//Initializes test instance of PostgresStat
// Important to not connect to database
func initPostgresStat() *PostgresStat {
	syscall.Dup2(int(logFile.Fd()), 2)

	s := new(PostgresStat)
	s.db = &testPostgresDB{
		Logger: log.New(os.Stderr, "TESTING LOG: ", 0),
	}
	s.PGDATA = "/data/pgsql"
	s.m = metrics.NewMetricContext("system")
	s.Modes = make(map[string]*ModeMetrics)
	s.DBs = make(map[string]*DBMetrics)
	s.Metrics = PostgresStatMetricsNew(s.m)
	s.dbLock = &sync.Mutex{}
	s.modeLock = &sync.Mutex{}
	s.pidCol = "procpid"
	s.queryCol = "current_query"
	s.idleCol = s.queryCol
	s.idleStr = "<IDLE>"

	return s
}

//checks ressults between expected and actual metrics gathered
func checkResults() string {
	for metric, expected := range expectedValues {
		switch m := metric.(type) {
		case *metrics.Counter:
			val, ok := expected.(uint64)
			if !ok {
				return "unexpected type"
			}
			if m.Get() != val {
				return ("unexpected value - got: " +
					strconv.FormatInt(int64(m.Get()), 10) + " but wanted " + strconv.FormatInt(int64(val), 10))
			}
		case *metrics.Gauge:
			val, ok := expected.(float64)
			if !ok {
				return "unexpected type"
			}
			if m.Get() != val {
				return ("unexpected value - got: " +
					strconv.FormatFloat(float64(m.Get()), 'f', 5, 64) + " but wanted " +
					strconv.FormatFloat(float64(val), 'f', 5, 64))
			}
		case string:
			val, ok := expected.(string)
			if !ok {
				return "unexpected type"
			}
			if m != val {
				return ("unexpected value - got: " + m + " but wated " + val)
			}
		}
	}
	return ""
}

//TestBasic parsing of all fields.
//Most metrics are simple parsing of strings to ints/floats.
//More complex string manipulations are further tested in
//later test functions.
func TestBasic(t *testing.T) {
	//initialize PostgresStat
	s := initPostgresStat()
	//set desired test output
	testquerycol = map[string]map[string][]string{
		uptimeQuery: map[string][]string{
			"uptime": []string{"15110"},
		},
		versionQuery: map[string][]string{
			"version": []string{"PostgreSQL 9.1.5 x86_64-linux-gnu"},
		},
		tpsQuery: map[string][]string{
			"tps": []string{"15122"},
		},
		cacheInfoQuery: map[string][]string{
			"block_reads_disk":  []string{"4"},
			"block_reads_cache": []string{"6"},
		},
		commitRatioQuery: map[string][]string{
			"commit_ratio": []string{"0.5"},
		},
		walKeepSegmentsQuery: map[string][]string{
			"setting": []string{"15.210"},
		},
		sessionMaxQuery: map[string][]string{
			"setting": []string{"200"},
		},
		fmt.Sprintf(sessionQuery, s.idleCol, s.idleStr, s.idleCol, s.idleStr): map[string][]string{
			"idle":   []string{"40"},
			"active": []string{"60"},
		},
		fmt.Sprintf(oldestQuery, "xact_start", s.idleCol, s.idleStr, s.queryCol): map[string][]string{
			"oldest": []string{"15251"},
		},
		fmt.Sprintf(oldestQuery, "query_start", s.idleCol, s.idleStr, s.queryCol): map[string][]string{
			"oldest": []string{"15112"},
		},
		fmt.Sprintf(longEntriesQuery, "30", s.idleCol, s.idleStr): map[string][]string{
			"entries": []string{"1", "2", "3", "4", "5", "6", "7"},
		},
		fmt.Sprintf(lockWaitersQuery, s.queryCol, s.queryCol, s.pidCol, s.pidCol): map[string][]string{
			"waiters": []string{"1", "2", "3", "4", "5", "6", "7", "8"},
		},
		locksQuery: map[string][]string{
			"lock1": []string{"15213"},
			"lock2": []string{"15322"},
			"lock3": []string{"15396"},
		},
		fmt.Sprintf(vacuumsQuery, s.queryCol, s.queryCol): map[string][]string{
			s.queryCol: []string{
				"autovacuum: ", "VACUUM", "ANALYZE", "autovacuum:", "VACUUM",
			},
		},
		secondsBehindMasterQuery: map[string][]string{
			"seconds": []string{"15424"},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.Uptime:               uint64(15110),
		s.Metrics.Version:              float64(9.15),
		s.Metrics.TPS:                  uint64(15122),
		s.Metrics.BlockReadsDisk:       uint64(4),
		s.Metrics.BlockReadsCache:      uint64(6),
		s.Metrics.CacheHitPct:          float64(60),
		s.Metrics.CommitRatio:          float64(0.5),
		s.Metrics.WalKeepSegments:      float64(15.210),
		s.Metrics.SessionMax:           float64(200),
		s.Metrics.SessionCurrentTotal:  float64(100),
		s.Metrics.SessionBusyPct:       float64(60),
		s.Metrics.ConnMaxPct:           float64(50),
		s.Metrics.OldestTrxS:           float64(15251),
		s.Metrics.OldestQueryS:         float64(15112),
		s.Metrics.ActiveLongRunQueries: float64(7),
		s.Metrics.LockWaiters:          float64(8),
		s.Modes["lock1"].Locks:         float64(15213),
		s.Modes["lock2"].Locks:         float64(15322),
		s.Modes["lock3"].Locks:         float64(15396),
		s.Metrics.VacuumsAutoRunning:   float64(2),
		s.Metrics.VacuumsManualRunning: float64(3),
		s.Metrics.SecondsBehindMaster:  float64(15424),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestVersion1(t *testing.T) {
	//initialize PostgresStat
	s := initPostgresStat()
	//set desired test output
	testquerycol = map[string]map[string][]string{
		versionQuery: map[string][]string{
			"version": []string{"PostgreSQL 9.1.5 x86_64-linux-gnu"},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.Version: float64(9.15),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestVersion2(t *testing.T) {
	//initialize PostgresStat
	s := initPostgresStat()
	//set desired test output
	testquerycol = map[string]map[string][]string{
		versionQuery: map[string][]string{
			"version": []string{"PostgreSQL 9.22.5 x86_64-linux-gnu"},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.Version: float64(9.225),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestVersion3(t *testing.T) {
	//initialize PostgresStat
	s := initPostgresStat()
	//set desired test output
	testquerycol = map[string]map[string][]string{
		versionQuery: map[string][]string{
			"version": []string{"PostgreSQL 9.3.43 x86_64-linux-gnu"},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.Version: float64(9.343),
		s.pidCol:          "pid",
		s.queryCol:        "query",
		s.idleCol:         "state",
		s.idleStr:         "idle",
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestVacuums1(t *testing.T) {
	//initialize PostgresStat
	s := initPostgresStat()
	//set desired test output
	testquerycol = map[string]map[string][]string{
		fmt.Sprintf(vacuumsQuery, s.queryCol, s.queryCol): map[string][]string{
			s.queryCol: []string{
				"autovacuum: ", "VACUUM", "ANALYZE", "autovacuum:", "VACUUM",
			},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.VacuumsAutoRunning:   float64(2),
		s.Metrics.VacuumsManualRunning: float64(3),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestVacuums2(t *testing.T) {
	//initialize PostgresStat
	s := initPostgresStat()
	//set desired test output
	testquerycol = map[string]map[string][]string{
		fmt.Sprintf(vacuumsQuery, s.queryCol, s.queryCol): map[string][]string{
			s.queryCol: []string{},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.VacuumsAutoRunning:   float64(0),
		s.Metrics.VacuumsManualRunning: float64(0),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestVacuums3(t *testing.T) {
	//initialize PostgresStat
	s := initPostgresStat()
	//set desired test output
	testquerycol = map[string]map[string][]string{
		fmt.Sprintf(vacuumsQuery, s.queryCol, s.queryCol): map[string][]string{
			s.queryCol: []string{
				"autovacuum:", "autovacuum:", "autovacuum:", "autovacuum:",
			},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.VacuumsAutoRunning:   float64(4),
		s.Metrics.VacuumsManualRunning: float64(0),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestVacuums4(t *testing.T) {
	//initialize PostgresStat
	s := initPostgresStat()
	//set desired test output
	testquerycol = map[string]map[string][]string{
		fmt.Sprintf(vacuumsQuery, s.queryCol, s.queryCol): map[string][]string{
			s.queryCol: []string{
				"blah blah blah ", "VACUUM", "ANALYZE", "ANALYZE", "VACUUM",
			},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.VacuumsAutoRunning:   float64(0),
		s.Metrics.VacuumsManualRunning: float64(5),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestVacuums5(t *testing.T) {
	//initialize PostgresStat
	s := initPostgresStat()
	//set desired test output
	testquerycol = map[string]map[string][]string{
		fmt.Sprintf(vacuumsQuery, s.queryCol, s.queryCol): map[string][]string{
			s.queryCol: []string{
				"", "", "", "",
			},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.VacuumsAutoRunning:   float64(0),
		s.Metrics.VacuumsManualRunning: float64(0),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestSecondsBehindMaster1(t *testing.T) {
	//initialize PostgresStat
	s := initPostgresStat()
	testquerycol = map[string]map[string][]string{
		secondsBehindMasterQuery: map[string][]string{
			"seconds": []string{"15453"},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.SecondsBehindMaster: float64(15453),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestSecondsBehindMaster2(t *testing.T) {
	//initialize PostgresStat
	s := initPostgresStat()
	testquerycol = map[string]map[string][]string{
		secondsBehindMasterQuery: map[string][]string{
			"seconds": []string{""},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.SecondsBehindMaster: float64(0),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestSecondsBehindMaster3(t *testing.T) {
	//initialize PostgresStat
	s := initPostgresStat()
	testquerycol = map[string]map[string][]string{
		secondsBehindMasterQuery: map[string][]string{
			"seconds": []string{"0"},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.SecondsBehindMaster: float64(0),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestSlaveDelayBytes1(t *testing.T) {
	s := initPostgresStat()
	testquerycol = map[string]map[string][]string{
		delayBytesQuery: map[string][]string{
			"client_hostname":          []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
			"pg_current_xlog_location": []string{"0/0"},
			"write_location":           []string{"0/0"},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.SlaveBytesBehindMe:  float64(0),
		s.Metrics.SlavesConnectedToMe: float64(10),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestSlaveDelayBytes2(t *testing.T) {
	s := initPostgresStat()
	testquerycol = map[string]map[string][]string{
		delayBytesQuery: map[string][]string{
			"client_hostname":          []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
			"pg_current_xlog_location": []string{"0/ff"},
			"write_location":           []string{"0/0"},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.SlaveBytesBehindMe:  float64(255),
		s.Metrics.SlavesConnectedToMe: float64(10),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestSlaveDelayBytes3(t *testing.T) {
	s := initPostgresStat()
	testquerycol = map[string]map[string][]string{
		delayBytesQuery: map[string][]string{
			"client_hostname":          []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
			"pg_current_xlog_location": []string{"c8/96"},
			"write_location":           []string{"64/32"},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.SlaveBytesBehindMe:  float64(429496729600),
		s.Metrics.SlavesConnectedToMe: float64(10),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}

func TestSecurity1(t *testing.T) {
	s := initPostgresStat()
	testquerycol = map[string]map[string][]string{
		securityQuery: map[string][]string{
			"usename": []string{"1", "2", "3", "4", "5"},
		},
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	expectedValues = map[interface{}]interface{}{
		s.Metrics.UnsecureUsers: float64(5),
	}
	err := checkResults()
	if err != "" {
		t.Fatalf(err)
	}
}
