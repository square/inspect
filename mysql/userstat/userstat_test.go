//Copyright (c) 2014 Square, Inc
//
// Tests the metrics collecting functions for userstat.go.
// Tests do not connect to a database, dummy functions are
// used instead and return hard coded input. Testing connections
// to a database are done in mysqltools_test.go.
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

package userstat

import (
	"log"
	"os"
	"strconv"
	"sync"
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

	//Mapping of metric and its expected value
	// defined as map of interface{}->interface{} so
	// can switch between metrics.Gauge and metrics.Counter
	// and between float64 and uint64 easily
	expectedValues = map[interface{}]interface{}{}

	logFile, _ = os.OpenFile("./test.log", os.O_WRONLY|os.O_CREATE|os.O_SYNC, 0644)
)

//functions that behave like mysqltools but we can make it return whatever
func (s *testMysqlDB) QueryReturnColumnDict(query string) (map[string][]string, error) {
	return testquerycol[query], nil
}

func (s *testMysqlDB) QueryMapFirstColumnToRow(query string) (map[string][]string, error) {
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

func initMysqlStatTable() *MysqlStatTables {
	syscall.Dup2(int(logFile.Fd()), 2)
	s := new(MysqlStatTables)
	s.Db = &testMysqlDB{
		Logger: log.New(os.Stderr, "TESTING LOG: ", log.Lshortfile),
	}
	s.nLock = &sync.Mutex{}

	s.M = metrics.NewMetricContext("system")
	s.Users = make(map[string]*MysqlUserStats)
	return s
}

//checkResults checks the results between
func checkResults() string {
	for metric, expected := range expectedValues {
		switch m := metric.(type) {
		case *metrics.Counter:
			val, ok := expected.(uint64) //assert expected val is uint64, this is really on the tester
			if !ok {
				return "unexpected type"
			}
			if m.Get() != val {
				return ("unexpected value - got: " +
					strconv.FormatInt(int64(m.Get()), 10) + " but wanted " +
					strconv.FormatInt(int64(val), 10))
			}
		case *metrics.Gauge:
			val, ok := expected.(float64) //assert expected val is float64
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

func TestUserStats(t *testing.T) {
	s := initMysqlStatTable()
	s.nLock.Lock()
	testquerycol = map[string]map[string][]string{
		// Test giving information for tables without the schema they
		// belong in being previously defined
		usrStatisticsQuery: map[string][]string{
			"user":                   []string{"u1", "u2", "u3", "u4", "u5"},
			"total_connections":      []string{"11", "12", "13", "14", "15"},
			"concurrent_connections": []string{"21", "22", "23", "24", "25"},
			"connected_time":         []string{"31", "32", "33", "34", "35"},
		},
	}
	s.nLock.Unlock()
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)

	s.nLock.Lock()
	expectedValues = map[interface{}]interface{}{
		s.Users["u1"].TotalConnections:      uint64(11),
		s.Users["u2"].TotalConnections:      uint64(12),
		s.Users["u3"].TotalConnections:      uint64(13),
		s.Users["u4"].TotalConnections:      uint64(14),
		s.Users["u5"].TotalConnections:      uint64(15),
		s.Users["u1"].ConcurrentConnections: uint64(21),
		s.Users["u2"].ConcurrentConnections: uint64(22),
		s.Users["u3"].ConcurrentConnections: uint64(23),
		s.Users["u4"].ConcurrentConnections: uint64(24),
		s.Users["u5"].ConcurrentConnections: uint64(25),
		s.Users["u1"].ConnectedTime:         uint64(31),
		s.Users["u2"].ConnectedTime:         uint64(32),
		s.Users["u3"].ConnectedTime:         uint64(33),
		s.Users["u4"].ConnectedTime:         uint64(34),
		s.Users["u5"].ConnectedTime:         uint64(35),
	}

	err := checkResults()
	s.nLock.Unlock()
	if err != "" {
		t.Error(err)
	}
}
