//Copyright (c) 2014 Square, Inc
//
// Tests the metrics collecting functions for mysqlstat.go.
// Tests do not connect to a database, dummy functions are
// used instead and return hard coded input. Testing connections
// to a database are done in mysqltools_test.go. Testing the
// "SHOW ENGINE INNODB" parser is also in mysqltools_test.go.
//
// Each test first sets input data, and uses Collect() to
// gather metrics rather than calling that metric's get function.
// This ensures that other functions still work on malformed or missing
// input, such as what would happen with an incorrect query.
// Testing the correctness of mysql queries should be done manually.
//
// Integration/Acceptance testing is harder and is avoided because
// creating and populating a fake database with the necessary information
// may be more trouble than is worth. Manual testing may be required for
// full acceptance tests.

package dbstat

import (
	"errors"
	"log"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/square/inspect/metrics"
)

type testMysqlDB struct {
	Logger *log.Logger
}

var (
	//testquerycol and testqueryrow map a query string to the desired test result
	//simulates QueryReturnColumnDict()
	testquerycol = map[string]map[string][]string{}

	//Simulates QueryMapFirstColumnToRow
	testqueryrow = map[string]map[string][]string{}

	//Hacky way of testing "SHOW GLOBAL STATUS"
	testglobalstats = []map[string][]string{}

	//Mapping of metric and its expected value
	// defined as map of interface{}->interface{} so
	// can switch between metrics.Gauge and metrics.Counter
	// and between float64 and uint64 easily
	expectedValues = map[interface{}]interface{}{}

	logFile, _ = os.OpenFile("./test.log", os.O_WRONLY|os.O_CREATE|os.O_SYNC, 0644)
)

//functions that behave like mysqltools but we can make it return whatever
func (s *testMysqlDB) QueryReturnColumnDict(query string) (map[string][]string, error) {
	if query == "SHOW ENGINE INNODB STATUS" {
		return nil, errors.New(" not checking innodb parser in this test")
	}

	return testquerycol[query], nil
}

func (s *testMysqlDB) QueryMapFirstColumnToRow(query string) (map[string][]string, error) {
	if query == "SHOW GLOBAL STATUS;" {
		result := testglobalstats[0]
		if len(testglobalstats) > 1 {
			testglobalstats = testglobalstats[1:]
		}
		return result, nil
	}
	return testquerycol[query], nil
}

func (s *testMysqlDB) Log(in interface{}) {
	s.Logger.Println(in)
}

func (s *testMysqlDB) Close() {
	return
}

func (s *testMysqlDB) SetMaxConnections(maxConns int) {
	return
}

func (s *testMysqlDB) QueryDb(query string) ([]string, [][]string, error) {
	return nil, nil, nil
}

func (s *testMysqlDB) DbExec(query string) (err error) {
	return nil
}

//initializes a test instance of MysqlStatDBs.
// instance does not connect with a db
func initMysqlStatDBs() *MysqlStatDBs {
	syscall.Dup2(int(logFile.Fd()), 2)
	s := new(MysqlStatDBs)
	s.Db = &testMysqlDB{
		Logger: log.New(os.Stderr, "TESTING LOG: ", log.Lshortfile),
	}
	s.Metrics = MysqlStatMetricsNew(metrics.NewMetricContext("system"))
	return s
}

//checkResults checks the results between
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
		}
	}
	return ""
}

// TestBasic parsing of all fields.
// Most metrics are simple parsing strings to ints/floats.
// More complex string manipulations are further tested in
// later test functions.
func TestBasic(t *testing.T) {
	//intitialize MysqlStatDBs
	s := initMysqlStatDBs()

	//set desired test output
	testquerycol = map[string]map[string][]string{
		//getSlaveStats()
		slaveQuery: map[string][]string{
			"Seconds_Behind_Master": []string{"8"},
			"Relay_Master_Log_File": []string{"some-name-bin.010"},
			"Exec_Master_Log_Pos":   []string{"79"},
			"Relay_Log_Space":       []string{"123"},
			"Master_Host":           []string{"abcdef"},
		},
		//getOldest
		oldestQuery: map[string][]string{
			"time": []string{"12345"},
		},
		// getBinlogFiles
		binlogQuery: map[string][]string{
			"Log_name":  []string{"binlog.00001", "binlog.00002", "binlog.00003", "binlog.00004"},
			"File_size": []string{"1", "10", "100", "1000"}, // sum = 1111
		},
		//getNumLongRunQueries
		longQuery: map[string][]string{
			"ID": []string{"1", "2", "3", "4", "5", "6", "7"},
		},
		//getVersion
		versionQuery: map[string][]string{
			"VERSION()": []string{"1.2.34"},
		},
		// getBinlogStats
		binlogStatsQuery: map[string][]string{
			"File":     []string{"mysql-bin.003"},
			"Position": []string{"73"},
		},
		// getStackedQueries
		stackedQuery: map[string][]string{
			"identical_queries_stacked": []string{"5", "4", "3"},
			"max_age":                   []string{"10", "9", "8"},
		},
		//getSessions
		sessionQuery1: map[string][]string{
			"max_connections": []string{"10"},
		},
		sessionQuery2: map[string][]string{
			"COMMAND": []string{"Sleep", "Connect", "Binlog Dump", "Binlog Dump GTID", "something else", "database stuff"},
			"USER":    []string{"unauthenticated", "user1", "user2", "user3", "user4", "user5"},
			"STATE":   []string{"statistics", "copying table", "Table Lock", "Table Lock", "Waiting for global read lock", "else"},
		},
		innodbQuery: map[string][]string{
			"Value": []string{"100"},
		},
		sslQuery: map[string][]string{
			"@@have_ssl": []string{"YES"},
		},
	}
	testglobalstats = []map[string][]string{map[string][]string{
		"Aborted_connects": []string{"51"},
		"Queries":          []string{"8"},
		"Uptime":           []string{"100"},
		"Threads_running":  []string{"5"},
	}}
	//expected results
	expectedValues = map[interface{}]interface{}{
		s.Metrics.SlaveSecondsBehindMaster: float64(8),
		s.Metrics.SlaveSeqFile:             float64(10),
		s.Metrics.SlavePosition:            uint64(79),
		s.Metrics.RelayLogSpace:            float64(123),
		s.Metrics.Queries:                  uint64(8),
		s.Metrics.Uptime:                   uint64(100),
		s.Metrics.ThreadsRunning:           float64(5),
		s.Metrics.MaxConnections:           float64(10),
		s.Metrics.CurrentSessions:          float64(6),
		s.Metrics.ActiveSessions:           float64(2),
		s.Metrics.UnauthenticatedSessions:  float64(1),
		s.Metrics.LockedSessions:           float64(0),
		s.Metrics.SessionTablesLocks:       float64(2),
		s.Metrics.SessionsCopyingToTable:   float64(1),
		s.Metrics.SessionsStatistics:       float64(1),
		s.Metrics.IdenticalQueriesStacked:  float64(5),
		s.Metrics.IdenticalQueriesMaxAge:   float64(10),
		s.Metrics.BinlogSeqFile:            float64(3),
		s.Metrics.BinlogPosition:           uint64(73),
		s.Metrics.Version:                  float64(1.234),
		s.Metrics.ActiveLongRunQueries:     float64(7),
		s.Metrics.BinlogSize:               float64(1111),
		s.Metrics.OldestQueryS:             float64(12345),
		s.Metrics.AbortedConnects:          uint64(51),
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)

	//check Results
	err := checkResults()
	if err != "" {
		t.Error(err)
	}

	if s.MasterHostname != "abcdef" {
		t.Error("MasterHost: expect abcdef, got " + s.MasterHostname)
	}
}

//test parsing of version
func TestVersion1(t *testing.T) {
	//intialize MysqlStatDBs
	s := initMysqlStatDBs()

	//set desired test result
	testquerycol = map[string]map[string][]string{
		versionQuery: map[string][]string{
			"VERSION()": []string{"123-456-789.0987"},
		},
	}
	//set expected result
	expectedValues = map[interface{}]interface{}{
		s.Metrics.Version: float64(123.4567890987),
	}
	//make sure to sleep for ~1 second before checking results
	// otherwise no metrics will be collected in time
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	//check results
	err := checkResults()
	if err != "" {
		t.Error(err)
	}
}

func TestVersion2(t *testing.T) {
	//intialize MysqlStatDBs
	s := initMysqlStatDBs()
	//repeat for different test results
	testquerycol = map[string]map[string][]string{
		versionQuery: map[string][]string{
			"VERSION()": []string{"123ABC456-abc-987"},
		},
	}
	expectedValues = map[interface{}]interface{}{
		s.Metrics.Version: float64(123456.987),
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	err := checkResults()
	if err != "" {
		t.Error(err)
	}
}

func TestVersion3(t *testing.T) {
	//intialize MysqlStatDBs
	s := initMysqlStatDBs()
	testquerycol = map[string]map[string][]string{
		versionQuery: map[string][]string{
			"VERSION()": []string{"abcdefg-123-456-qwerty"},
		},
	}
	expectedValues = map[interface{}]interface{}{
		s.Metrics.Version: float64(0.123456),
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	err := checkResults()
	if err != "" {
		t.Error(err)
	}
}

//Test Parsing of sessions query
func TestSessions(t *testing.T) {
	//initialize MysqlStatDBs
	s := initMysqlStatDBs()
	//set desired query output
	testquerycol = map[string]map[string][]string{
		sessionQuery1: map[string][]string{
			"max_connections": []string{"100"},
		},
		sessionQuery2: map[string][]string{
			"COMMAND": []string{"Sleep", "Connect", "Binlog Dump", "Binlog Dump GTID", "something else", "database stuff",
				"Sleep", "Sleep", "database stuff", "other things", "square", "square2"},
			"USER": []string{"unauthenticated", "user1", "user2", "user3", "user4", "user5",
				"also unauthenticated", "unauthenticated", "root", "root", "root", "root"},
			"STATE": []string{"statistics", "copying another table", "Table Lock", "Table Lock",
				"Waiting for global read lock", "else", "Table Lock", "Locked", "statistics",
				"statistics", "copying table also", "copying table also also"},
		},
	}
	//set expected values
	expectedValues = map[interface{}]interface{}{
		s.Metrics.MaxConnections:          float64(100),
		s.Metrics.CurrentSessions:         float64(12),
		s.Metrics.CurrentConnectionsPct:   float64(12),
		s.Metrics.ActiveSessions:          float64(6),
		s.Metrics.BusySessionPct:          float64(50),
		s.Metrics.UnauthenticatedSessions: float64(3),
		s.Metrics.LockedSessions:          float64(1),
		s.Metrics.SessionTablesLocks:      float64(3),
		s.Metrics.SessionGlobalReadLocks:  float64(1),
		s.Metrics.SessionsCopyingToTable:  float64(3),
		s.Metrics.SessionsStatistics:      float64(3),
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	err := checkResults()
	if err != "" {
		t.Error(err)
	}
}

// Test basic parsing of slave info query
func TestSlave1(t *testing.T) {
	//intitialize MysqlStatDBs
	s := initMysqlStatDBs()
	//set desired test output
	testquerycol = map[string]map[string][]string{
		//getSlaveStats()
		slaveQuery: map[string][]string{
			"Seconds_Behind_Master": []string{"80"},
			"Relay_Master_Log_File": []string{"some-name-bin.01345"},
			"Exec_Master_Log_Pos":   []string{"7"},
			"Relay_Log_Space":       []string{"2"},
		},
		slaveBackupQuery: map[string][]string{
			"count": []string{"0"},
		},
	}
	expectedValues = map[interface{}]interface{}{
		s.Metrics.SlaveSecondsBehindMaster: float64(80),
		s.Metrics.SlaveSeqFile:             float64(1345),
		s.Metrics.SlavePosition:            uint64(7),
		s.Metrics.ReplicationRunning:       float64(1),
		s.Metrics.RelayLogSpace:            float64(2),
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	err := checkResults()
	if err != "" {
		t.Error(err)
	}

	if s.MasterHostname != "" {
		t.Error("MasterHost: Expect empty string, got " + s.MasterHostname)
	}
}

// Test when slave is down and backup isn't running
func TestSlave2(t *testing.T) {
	//intitialize MysqlStatDBs
	s := initMysqlStatDBs()
	//set desired test output
	testquerycol = map[string]map[string][]string{
		//getSlaveStats()
		slaveQuery: map[string][]string{
			"Seconds_Behind_Master": []string{"NULL"},
			"Relay_Master_Log_File": []string{"some.name.bin.01345"},
			"Exec_Master_Log_Pos":   []string{"7"},
			"Relay_Log_Space":       []string{"0"},
		},
		slaveBackupQuery: map[string][]string{
			"count": []string{"0"},
		},
	}
	expectedValues = map[interface{}]interface{}{
		s.Metrics.SlaveSecondsBehindMaster: float64(-1),
		s.Metrics.SlaveSeqFile:             float64(1345),
		s.Metrics.SlavePosition:            uint64(7),
		s.Metrics.ReplicationRunning:       float64(-1),
		s.Metrics.RelayLogSpace:            float64(0),
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	err := checkResults()
	if err != "" {
		t.Error(err)
	}
}

// Test when slave is down and backup is running
func TestSlave3(t *testing.T) {
	//intitialize MysqlStatDBs
	s := initMysqlStatDBs()
	//set desired test output
	testquerycol = map[string]map[string][]string{
		//getSlaveStats()
		slaveQuery: map[string][]string{
			"Seconds_Behind_Master": []string{"NULL"},
			"Relay_Master_Log_File": []string{"some.name.bin.01345"},
			"Exec_Master_Log_Pos":   []string{"7"},
			"Relay_Log_Space":       []string{"0"},
		},
		slaveBackupQuery: map[string][]string{
			"count": []string{"1"},
		},
	}
	expectedValues = map[interface{}]interface{}{
		s.Metrics.SlaveSecondsBehindMaster: float64(-1),
		s.Metrics.SlaveSeqFile:             float64(1345),
		s.Metrics.SlavePosition:            uint64(7),
		s.Metrics.ReplicationRunning:       float64(1),
		s.Metrics.RelayLogSpace:            float64(0),
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	err := checkResults()
	if err != "" {
		t.Error(err)
	}
}

func TestUnsecureUsers1(t *testing.T) {
	s := initMysqlStatDBs()
	testquerycol = map[string]map[string][]string{
		securityQuery: map[string][]string{
			"COUNT(*)": []string{"8"},
		},
	}
	expectedValues = map[interface{}]interface{}{
		s.Metrics.UnsecureUsers: float64(8),
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	err := checkResults()
	if err != "" {
		t.Error(err)
	}
}

func TestUnsecureUsers2(t *testing.T) {
	s := initMysqlStatDBs()
	testquerycol = map[string]map[string][]string{
		securityQuery: map[string][]string{
			"COUNT(*)": []string{"0"},
		},
	}
	expectedValues = map[interface{}]interface{}{
		s.Metrics.UnsecureUsers: float64(0),
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	err := checkResults()
	if err != "" {
		t.Error(err)
	}
}

func TestReadOnly(t *testing.T) {
	s := initMysqlStatDBs()
	testquerycol = map[string]map[string][]string{
		readOnlyQuery: map[string][]string{
			"@@read_only": []string{"0"},
		},
		superReadOnlyQuery: map[string][]string{
			"@@super_read_only": []string{"1"},
		},
	}
	expectedValues = map[interface{}]interface{}{
		s.Metrics.IsReadOnly:      float64(0),
		s.Metrics.IsSuperReadOnly: float64(1),
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	err := checkResults()
	if err != "" {
		t.Error(err)
	}
}

func TestGetQueriesAndUptime1(t *testing.T) {
	s := initMysqlStatDBs()
	testglobalstats = []map[string][]string{
		map[string][]string{
			"Queries": []string{"0"},
			"Uptime":  []string{"1"},
		},
	}
	q, u, err := s.getQueriesAndUptime()
	if err != nil {
		t.Error("Values not collected properly: err")
	}
	if q != 0.0 {
		t.Error("Values not collected properly: q")
	}
	if u != 1.0 {
		t.Error("Values not collected properly: u")
	}
}

func TestGetQueriesPerSecond(t *testing.T) {
	s := initMysqlStatDBs()
	testglobalstats = []map[string][]string{
		map[string][]string{
			"Queries": []string{"0"},
			"Uptime":  []string{"1"},
		},
		map[string][]string{
			"Queries": []string{"100"},
			"Uptime":  []string{"2"},
		},
	}
	expectedValues = map[interface{}]interface{}{
		s.Metrics.QueriesPerSecond: float64(100),
	}
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	err := checkResults()
	if err != "" {
		t.Error(err)
	}
}
