//Copyright (c) 2015 Square, Inc
//
// Tests the metrics collecting functions for tablestat.go.
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

package tablestat

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
	s.DBs = make(map[string]*DBStats)
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

// TestBasic parsing of all fields.
// Most metrics are simple parsing strings to ints/floats.
// More complex string manipulations are further tested in
// later test functions.
func TestBasic(t *testing.T) {

	s := initMysqlStatTable()
	s.nLock.Lock()
	testquerycol = map[string]map[string][]string{
		innodbMetadataCheck: map[string][]string{
			"innodb_stats_on_metadata": []string{"0"},
		},
		//this particular query uses MapFirstColumnToRow
		// so each database name points to its size
		dbSizesQuery: map[string][]string{
			"db1": []string{"100"},
			"db2": []string{"200"},
			"db3": []string{"300"},
		},
		tblSizesQuery: map[string][]string{
			"tbl":            []string{"t1", "t2", "t3", "t1", "t2", "t1", "t1"},
			"db":             []string{"db1", "db1", "db1", "db2", "db2", "db3", "db4"},
			"tbl_size_bytes": []string{"1", "2", "3", "4", "5", "6", "7"},
		},
		tblStatisticsQuery: map[string][]string{
			"db":                     []string{"db1", "db1", "db2", "db3", "db5"},
			"tbl":                    []string{"t1", "t2", "t1", "t2", "t1"},
			"rows_read":              []string{"11", "12", "13", "14", "15"},
			"rows_changed":           []string{"21", "22", "23", "24", "25"},
			"rows_changed_x_indexes": []string{"31", "32", "33", "34", "35"},
		},
	}
	s.nLock.Unlock()
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)

	// define expected values after running collect so that databases and
	// tables are instantiated
	s.nLock.Lock()
	expectedValues = map[interface{}]interface{}{
		s.DBs["db1"].Metrics.SizeBytes:                float64(100),
		s.DBs["db2"].Metrics.SizeBytes:                float64(200),
		s.DBs["db3"].Metrics.SizeBytes:                float64(300),
		s.DBs["db1"].Tables["t1"].SizeBytes:           float64(1),
		s.DBs["db1"].Tables["t2"].SizeBytes:           float64(2),
		s.DBs["db1"].Tables["t3"].SizeBytes:           float64(3),
		s.DBs["db2"].Tables["t1"].SizeBytes:           float64(4),
		s.DBs["db2"].Tables["t2"].SizeBytes:           float64(5),
		s.DBs["db3"].Tables["t1"].SizeBytes:           float64(6),
		s.DBs["db4"].Tables["t1"].SizeBytes:           float64(7),
		s.DBs["db1"].Tables["t1"].RowsRead:            uint64(11),
		s.DBs["db1"].Tables["t2"].RowsRead:            uint64(12),
		s.DBs["db2"].Tables["t1"].RowsRead:            uint64(13),
		s.DBs["db3"].Tables["t2"].RowsRead:            uint64(14),
		s.DBs["db5"].Tables["t1"].RowsRead:            uint64(15),
		s.DBs["db1"].Tables["t1"].RowsChanged:         uint64(21),
		s.DBs["db1"].Tables["t2"].RowsChanged:         uint64(22),
		s.DBs["db2"].Tables["t1"].RowsChanged:         uint64(23),
		s.DBs["db3"].Tables["t2"].RowsChanged:         uint64(24),
		s.DBs["db5"].Tables["t1"].RowsChanged:         uint64(25),
		s.DBs["db1"].Tables["t1"].RowsChangedXIndexes: uint64(31),
		s.DBs["db1"].Tables["t2"].RowsChangedXIndexes: uint64(32),
		s.DBs["db2"].Tables["t1"].RowsChangedXIndexes: uint64(33),
		s.DBs["db3"].Tables["t2"].RowsChangedXIndexes: uint64(34),
		s.DBs["db5"].Tables["t1"].RowsChangedXIndexes: uint64(35),
	}
	err := checkResults()
	s.nLock.Unlock()
	if err != "" {
		t.Error(err)
	}
}

func TestDBSizes(t *testing.T) {

	s := initMysqlStatTable()
	s.nLock.Lock()
	testquerycol = map[string]map[string][]string{
		innodbMetadataCheck: map[string][]string{
			"innodb_stats_on_metadata": []string{"0"},
		},
		//this particular query uses MapFirstColumnToRow
		// so each database name points to its size
		dbSizesQuery: map[string][]string{
			"db1": []string{"100"},
			"db2": []string{"200"},
			"db3": []string{"300"},
			"db4": []string{"400"},
			"db5": []string{"500"},
			"db6": []string{"600"},
		},
	}
	s.nLock.Unlock()
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	s.nLock.Lock()
	expectedValues = map[interface{}]interface{}{
		s.DBs["db1"].Metrics.SizeBytes: float64(100),
		s.DBs["db2"].Metrics.SizeBytes: float64(200),
		s.DBs["db3"].Metrics.SizeBytes: float64(300),
		s.DBs["db4"].Metrics.SizeBytes: float64(400),
		s.DBs["db5"].Metrics.SizeBytes: float64(500),
		s.DBs["db6"].Metrics.SizeBytes: float64(600),
	}
	err := checkResults()
	s.nLock.Unlock()
	if err != "" {
		t.Error(err)
	}
}

func TestTableSizes(t *testing.T) {

	s := initMysqlStatTable()
	s.nLock.Lock()
	testquerycol = map[string]map[string][]string{
		innodbMetadataCheck: map[string][]string{
			"innodb_stats_on_metadata": []string{"0"},
		},
		// Test giving information for tables without the schema they
		// belong in being previously defined
		tblSizesQuery: map[string][]string{
			"tbl":            []string{"t1", "t2", "t3", "t1", "t2", "t1", "t1"},
			"db":             []string{"db1", "db1", "db1", "db2", "db2", "db3", "db4"},
			"tbl_size_bytes": []string{"1", "2", "3", "4", "5", "6", "7"},
		},
	}
	s.nLock.Unlock()
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)

	s.nLock.Lock()
	expectedValues = map[interface{}]interface{}{
		s.DBs["db1"].Tables["t1"].SizeBytes: float64(1),
		s.DBs["db1"].Tables["t2"].SizeBytes: float64(2),
		s.DBs["db1"].Tables["t3"].SizeBytes: float64(3),
		s.DBs["db2"].Tables["t1"].SizeBytes: float64(4),
		s.DBs["db2"].Tables["t2"].SizeBytes: float64(5),
		s.DBs["db3"].Tables["t1"].SizeBytes: float64(6),
		s.DBs["db4"].Tables["t1"].SizeBytes: float64(7),
	}
	err := checkResults()
	s.nLock.Unlock()
	if err != "" {
		t.Error(err)
	}
}

func TestTableStats(t *testing.T) {

	s := initMysqlStatTable()
	s.nLock.Lock()
	testquerycol = map[string]map[string][]string{
		// Test giving information for tables without the schema they
		// belong in being previously defined
		tblStatisticsQuery: map[string][]string{
			"db":                     []string{"db1", "db1", "db2", "db3", "db5"},
			"tbl":                    []string{"t1", "t2", "t1", "t2", "t1"},
			"rows_read":              []string{"11", "12", "13", "14", "15"},
			"rows_changed":           []string{"21", "22", "23", "24", "25"},
			"rows_changed_x_indexes": []string{"31", "32", "33", "34", "35"},
		},
	}
	s.nLock.Unlock()
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)

	s.nLock.Lock()
	expectedValues = map[interface{}]interface{}{
		s.DBs["db1"].Tables["t1"].RowsRead:            uint64(11),
		s.DBs["db1"].Tables["t2"].RowsRead:            uint64(12),
		s.DBs["db2"].Tables["t1"].RowsRead:            uint64(13),
		s.DBs["db3"].Tables["t2"].RowsRead:            uint64(14),
		s.DBs["db5"].Tables["t1"].RowsRead:            uint64(15),
		s.DBs["db1"].Tables["t1"].RowsChanged:         uint64(21),
		s.DBs["db1"].Tables["t2"].RowsChanged:         uint64(22),
		s.DBs["db2"].Tables["t1"].RowsChanged:         uint64(23),
		s.DBs["db3"].Tables["t2"].RowsChanged:         uint64(24),
		s.DBs["db5"].Tables["t1"].RowsChanged:         uint64(25),
		s.DBs["db1"].Tables["t1"].RowsChangedXIndexes: uint64(31),
		s.DBs["db1"].Tables["t2"].RowsChangedXIndexes: uint64(32),
		s.DBs["db2"].Tables["t1"].RowsChangedXIndexes: uint64(33),
		s.DBs["db3"].Tables["t2"].RowsChangedXIndexes: uint64(34),
		s.DBs["db5"].Tables["t1"].RowsChangedXIndexes: uint64(35),
	}
	err := checkResults()
	s.nLock.Unlock()
	if err != "" {
		t.Error(err)
	}
}

//Because innodb stats on metadata is being collected,
//metrics collector should not collect these metrics
func TestNoSizes(t *testing.T) {

	s := initMysqlStatTable()
	s.nLock.Lock()
	testquerycol = map[string]map[string][]string{
		innodbMetadataCheck: map[string][]string{
			"innodb_stats_on_metadata": []string{"1"},
		},
		dbSizesQuery: map[string][]string{
			"db1": []string{"100"},
			"db2": []string{"200"},
			"db3": []string{"300"},
		},
		tblSizesQuery: map[string][]string{
			"tbl":            []string{"t1", "t2", "t3", "t1", "t2", "t1", "t1"},
			"db":             []string{"db1", "db1", "db1", "db2", "db2", "db3", "db4"},
			"tbl_size_bytes": []string{"1", "2", "3", "4", "5", "6", "7"},
		},
	}
	s.nLock.Unlock()
	s.Collect()
	time.Sleep(time.Millisecond * 1000 * 1)
	s.nLock.Lock()
	defer s.nLock.Unlock()
	_, ok := s.DBs["db1"]
	if ok {
		t.Error("found database, but should not have")
	}
	_, ok = s.DBs["db2"]
	if ok {
		t.Error("found database, but should not have")
	}
	_, ok = s.DBs["db2"]
	if ok {
		t.Error("found database, but should not have")
	}
}
