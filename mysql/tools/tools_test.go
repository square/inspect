//Copyright (c) 2014 Square, Inc
//
// Testing for mysqltools.go.
// These test connectivity and interactions with a mysql database.
// These do not test using the same queries used for metrics collection.
// The tmpmysqld package is used to make temporary test databases
// to connect to. The package is used to test that the query functions
// work as intended and that their results are formatted as will
// be used in mysqlstat.go and mysqlstat-tables.go.
//
// Since these tests make use of a temporary mysql instance. Connections
// to permanent databases requiring passwords should be tested manually.
//
// Integration/Acceptance testing is harder and is avoided because
// creating and populating a fake database with the necessary information
// may be more trouble than is worth. Manual testing may be required for
// full acceptance tests.

package tools

import (
	"testing"

	"github.com/codahale/tmpmysqld"
)

var (
	testInnodbStats = InnodbStats{}
	expectedValues  = map[interface{}]interface{}{}
)

const (
	prefix     = "./testfiles/"
	fakeDBName = "foobar"
)

//initialize test mysql instance and populate with data
func initDB(t testing.TB) mysqlDB {
	server, err := tmpmysql.NewMySQLServer("inspect_mysql_test")
	if err != nil {
		t.Fatal(err)
	}

	test := new(mysqlDB)
	test.db = server.DB
	test.dsnString = "/inspect_mysql_test"

	commands := []string{`
    CREATE TEMPORARY TABLE people (
        name VARCHAR(50) NOT NULL,
        age INT UNSIGNED NOT NULL,
        birthday VARCHAR(50) NOT NULL);`,
		`
    INSERT INTO people
        (name, age, birthday)
        VALUES
        ('alice', 20, 'Jan');`,
		`
    INSERT INTO people
        (name, age, birthday)
        VALUES
        ('bob', 21, 'Feb');`,
		`
    INSERT INTO people
        (name, age, birthday)
        VALUES
        ('charlie', 22, 'Mar');`,
		`
    INSERT INTO people
        (name, age, birthday)
        VALUES
        ('david', 23, 'Apr');`}
	for _, cmd := range commands {
		_, err := test.db.Exec(cmd)
		if err != nil {
			t.Fatal(err)
		}
	}
	return *test
}

//tests string manipulation of making dsn string
func TestMakeDsn1(t *testing.T) {
	dsn := map[string]string{
		"user":     "brian",
		"password": "secret...shhh!",
		"dbname":   "mysqldb",
	}
	expected := "brian:secret...shhh!@/mysqldb?timeout=30s"
	result := makeDsn(dsn)
	if result != expected {
		t.Error("Incorrect result, expected: " + expected + " but got: " + result)
	}
}

func TestMakeDsn2(t *testing.T) {
	dsn := map[string]string{
		"dbname": "mysqldb",
	}
	expected := "/mysqldb?timeout=30s"
	result := makeDsn(dsn)
	if result != expected {
		t.Error("Incorrect result, expected: " + expected + " but got: " + result)
	}
}

func TestMakeDsn3(t *testing.T) {
	dsn := map[string]string{
		"user":     "brian",
		"password": "secret...shhh!",
		"host":     "unix(mysql.sock)",
		"dbname":   "mysqldb",
	}
	expected := "brian:secret...shhh!@unix(mysql.sock)/mysqldb?timeout=30s"
	result := makeDsn(dsn)
	if result != expected {
		t.Error("Incorrect result, expected: " + expected + " but got: " + result)
	}
}

//test that the correct data is returned,
// as well as test that the ordering is preserved
func TestMakeQuery1(t *testing.T) {
	testdb := initDB(t)
	defer testdb.db.Close()

	cols, data, err := testdb.makeQuery("SELECT name FROM people;")
	if err != nil {
		t.Error(err)
	}
	//its important to test lengths so tests don't panic and exit early
	if len(cols) != 1 || len(data) != 1 || len(data[0]) != 4 {
		t.Error("Unexpected data returned")
	}
	if cols[0] != "name" || data[0][0] != "alice" ||
		data[0][1] != "bob" || data[0][2] != "charlie" || data[0][3] != "david" {
		t.Error("Unexpected data returned")
	}
}

func TestMakeQuery2(t *testing.T) {
	testdb := initDB(t)
	defer testdb.db.Close()

	cols, data, err := testdb.makeQuery("SELECT name, age FROM people;")
	if err != nil {
		t.Error(err)
	}
	//its important to test lengths so tests don't panic and exit early
	if len(cols) != 2 || len(data) != 2 || len(data[0]) != 4 || len(data[1]) != 4 {
		t.Error("Unexpected data size returned")
	}
	if cols[0] != "name" || data[0][0] != "alice" ||
		data[0][1] != "bob" || data[0][2] != "charlie" || data[0][3] != "david" ||
		data[1][0] != "20" || data[1][1] != "21" || data[1][2] != "22" ||
		data[1][3] != "23" {
		t.Error("Unexpected data returned")
	}
}

//after ensuring TestMakeQuery1 and TestMakeQuery2 are correct,
//can test QueryReturnColumnDict and QueryMapFirstColumnToRow.
//these tests ensure that the results returned to mysqlstat and mysqlstattables
//are formatted as expected.
func TestQueryReturnColumnDict1(t *testing.T) {
	testdb := initDB(t)
	defer testdb.db.Close()

	res, err := testdb.QueryReturnColumnDict("SELECT name FROM people;")
	if err != nil {
		t.Error(err)
	}
	data, ok := res["name"]
	//its important to test lengths so tests don't panic and exit early
	if !ok || len(data) != 4 {
		t.Error("Unexpected data returned")
	}
	if data[0] != "alice" || data[1] != "bob" ||
		data[2] != "charlie" || data[3] != "david" {
		t.Error("Unexpected data returned")
	}
}

func TestQueryReturnColumnDict2(t *testing.T) {
	testdb := initDB(t)
	defer testdb.db.Close()

	res, err := testdb.QueryReturnColumnDict("SELECT name, birthday FROM people;")
	if err != nil {
		t.Error(err)
	}
	names, namesok := res["name"]
	bday, bdayok := res["birthday"]
	//its important to test lengths so tests don't panic and exit early
	if !namesok || !bdayok || len(names) != 4 || len(bday) != 4 {
		t.Error("Unexpected data returned")
	}
	if names[0] != "alice" || names[1] != "bob" ||
		names[2] != "charlie" || names[3] != "david" {
		t.Error("Unexpected name returned")
	}
	if bday[0] != "Jan" || bday[1] != "Feb" ||
		bday[2] != "Mar" || bday[3] != "Apr" {
		t.Error("Unexpected birthday returned")
	}
}

func TestQueryMapFirstColumnToRow1(t *testing.T) {
	testdb := initDB(t)
	defer testdb.db.Close()

	res, err := testdb.QueryMapFirstColumnToRow("SELECT name, birthday FROM people;")
	if err != nil {
		t.Error(err)
	}
	alice, aliceok := res["alice"]
	bob, bobok := res["bob"]
	charlie, charlieok := res["charlie"]
	david, davidok := res["david"]
	if !aliceok || !bobok || !charlieok || !davidok {
		t.Error("Unexpected data returned")
	}
	//its important to test lengths so tests don't panic and exit early
	if len(alice) != 1 || len(alice) != 1 || len(alice) != 1 || len(alice) != 1 {
		t.Error("Unexpected data size returned")
	}
	if alice[0] != "Jan" || bob[0] != "Feb" ||
		charlie[0] != "Mar" || david[0] != "Apr" {
		t.Error("Unexpected birthday returned")
	}
}

func TestQueryMapFirstColumnToRow2(t *testing.T) {
	testdb := initDB(t)
	defer testdb.db.Close()

	res, err := testdb.QueryMapFirstColumnToRow("SELECT name, birthday, age FROM people;")
	if err != nil {
		t.Error(err)
	}
	alice, aliceok := res["alice"]
	bob, bobok := res["bob"]
	charlie, charlieok := res["charlie"]
	david, davidok := res["david"]
	if !aliceok || !bobok || !charlieok || !davidok {
		t.Error("Unexpected data returned")
	}
	//its important to test lengths so tests don't panic and exit early
	if len(alice) != 2 || len(alice) != 2 || len(alice) != 2 || len(alice) != 2 {
		t.Error("Unexpected data size returned")
	}
	if alice[0] != "Jan" || bob[0] != "Feb" ||
		charlie[0] != "Mar" || david[0] != "Apr" {
		t.Error("Unexpected birthday returned")
	}
	if alice[1] != "20" || bob[1] != "21" ||
		charlie[1] != "22" || david[1] != "23" {
		t.Error("Unexpected age returned")
	}
}

//Tests a "bad" connection to the database. On losing a connection
//to a mysql db, metrics collector should retry connecting to database.
func TestBadConnection1(t *testing.T) {
	testdb := initDB(t)
	defer testdb.db.Close()

	_, _, err := testdb.queryDb("SELECT * FROM people;")
	if err != nil {
		t.Error(err)
	}
	//close the connection to the db to ~simulate (kinda)~ a lost connection
	testdb.db.Close()

	_, _, err = testdb.queryDb("SELECT * FROM people;")
	if err != nil {
		t.Error("failed to reconnect: %v", err)
	}
}

//Tests regex's and parsing for SHOW ENGINE INDDOB query
func TestParseFileIO(t *testing.T) {
	idb := new(InnodbStats)
	idb.Metrics = make(map[string]string)
	blob := `
I/O thread 0 state: waiting for i/o request (insert buffer thread)
I/O thread 9 state: waiting for i/o request (write thread)
Pending normal aio reads: 0 [0, 0, 0, 0] , aio writes: 0 [0, 0, 0, 0] ,
 ibuf aio reads: 0, log i/o's: 0, sync i/o's: 0
Pending flushes (fsync) log: 0; buffer pool: 0
1597 OS file reads, 423166 OS file writes, 367474 OS fsyncs
0.00 reads/s, 0 avg bytes/read, 1.48 writes/s, 0.89 fsyncs/s`
	idb.parseFileIO(blob)
	expectedValues := map[string]string{
		"OS_file_reads":      "1597",
		"OS_file_writes":     "423166",
		"reads_per_s":        "0.00",
		"avg_bytes_per_read": "0",
		"writes_per_s":       "1.48",
		"fsyncs_per_s":       "0.89",
	}
	for key, val := range expectedValues {
		if idb.Metrics[key] != val {
			t.Error(key + " not parsed correctly. Expected: " + val + ", Got: " + idb.Metrics[key])
		}
	}
}

func TestParseLog(t *testing.T) {
	idb := new(InnodbStats)
	idb.Metrics = make(map[string]string)
	blob := `
Log sequence number 139401311
Log flushed up to   139401312
Pages flushed up to 139401313
Last checkpoint at  139401310
Max checkpoint age    80826164
Checkpoint age target 78300347
Modified age          0
Checkpoint age        1
2 pending log writes, 3 pending chkp writes
277124 log i/o's done, 0.41 log i/o's/second`
	idb.parseLog(blob)
	expectedValues := map[string]string{
		"log_sequence_number":   "139401311",
		"log_flushed_up_to":     "139401312",
		"pages_flushed_up_to":   "139401313",
		"last_checkpoint_at":    "139401310",
		"max_checkpoint_age":    "80826164",
		"checkpoint_age_target": "78300347",
		"modified_age":          "0",
		"checkpoint_age":        "1",
		"pending_log_writes":    "2",
		"pending_chkp_writes":   "3",
		"log_io_done":           "277124",
		"log_io_per_sec":        "0.41",
	}
	for key, val := range expectedValues {
		if idb.Metrics[key] != val {
			t.Error(key + " not parsed correctly. Expected: " + val + ", Got: " + idb.Metrics[key])
		}
	}
}

func TestParseBufferPoolAndMem1(t *testing.T) {
	idb := new(InnodbStats)
	idb.Metrics = make(map[string]string)
	blob := `
Total memory allocated 137363456; in additional pool allocated 0
Total memory allocated by read views 472
Internal hash tables (constant factor + variable factor)
    Adaptive hash index 2250352     (2213368 + 36984)
    Page hash           139112 (buffer pool 0 only)
    Dictionary cache    5771169     (554768 + 5216401)
    File system         1053936     (812272 + 241664)
    Lock system         335128  (332872 + 2256)
    Recovery system     0   (0 + 0)
Dictionary memory allocated 5216401
Buffer pool size        8191
Buffer pool size, bytes 134201344
Free buffers            1024
Database pages          7165
Old database pages      2624
Modified db pages       1
Pending reads 2
Pending writes: LRU 3, flush list 0, single page 0
Pages made young 856, not young 24422
0.00 youngs/s, 0.00 non-youngs/s
Pages read 534, created 12629, written 157429
0.00 reads/s, 0.00 creates/s, 0.96 writes/s
Buffer pool hit rate 1000 / 1000, young-making rate 0 / 1000 not 0 / 1000
Pages read ahead 0.00/s, evicted without access 0.00/s, Random read ahead 0.00/s
LRU len: 7165, unzip_LRU len: 0
I/O sum[42]:cur[0], unzip sum[0]:cur[0]`
	idb.parseBufferPoolAndMem(blob)
	expectedValues := map[string]string{
		"total_mem":                   "137363456",
		"total_mem_by_read_views":     "472",
		"adaptive_hash":               "2250352",
		"page_hash":                   "139112",
		"dictionary_cache":            "5771169",
		"file_system":                 "1053936",
		"lock_system":                 "335128",
		"recovery_system":             "0",
		"dictionary_memory_allocated": "5216401",
		"buffer_pool_size":            "8191",
		"free_buffers":                "1024",
		"database_pages":              "7165",
		"old_database_pages":          "2624",
		"modified_db_pages":           "1",
		"pending_writes_lru":          "3",
		"pages_made_young":            "856",
		"buffer_pool_hit_rate":        "1",
		"cache_hit_pct":               "100",
	}
	for key, val := range expectedValues {
		if idb.Metrics[key] != val {
			t.Error(key + " not parsed correctly. Expected: " + val + ", Got: " + idb.Metrics[key])
		}
	}
}

func TestParseBufferPoolAndMem2(t *testing.T) {
	idb := new(InnodbStats)
	idb.Metrics = make(map[string]string)
	blob := `
Total memory allocated 137363456; in additional pool allocated 0
Total memory allocated by read views 472
Internal hash tables (constant factor + variable factor)
    Adaptive hash index 2250352  
    Page hash           139112 
    Dictionary cache    5771169     
    File system         1053936    
    Lock system         335128  
    Recovery system     0   
Dictionary memory allocated 5216401
Buffer pool size        8191
Buffer pool size, bytes 134201344
Free buffers            1024
Database pages          7165
Old database pages      2624
Modified db pages       1
Pending reads 2
Pending writes: LRU 3, flush list 0, single page 0
Pages made young 856, not young 24422
0.00 youngs/s, 0.00 non-youngs/s
Pages read 534, created 12629, written 157429
0.00 reads/s, 0.00 creates/s, 0.96 writes/s
Buffer pool hit rate 500 / 1000, young-making rate 0 / 1000 not 0 / 1000
Pages read ahead 0.00/s, evicted without access 0.00/s, Random read ahead 0.00/s
LRU len: 7165, unzip_LRU len: 0
I/O sum[42]:cur[0], unzip sum[0]:cur[0]`
	idb.parseBufferPoolAndMem(blob)
	expectedValues := map[string]string{
		"total_mem":                   "137363456",
		"total_mem_by_read_views":     "472",
		"adaptive_hash":               "2250352",
		"page_hash":                   "139112",
		"dictionary_cache":            "5771169",
		"file_system":                 "1053936",
		"lock_system":                 "335128",
		"recovery_system":             "0",
		"dictionary_memory_allocated": "5216401",
		"buffer_pool_size":            "8191",
		"free_buffers":                "1024",
		"database_pages":              "7165",
		"old_database_pages":          "2624",
		"modified_db_pages":           "1",
		"pending_writes_lru":          "3",
		"pages_made_young":            "856",
		"buffer_pool_hit_rate":        "0.5",
		"cache_hit_pct":               "50",
	}
	for key, val := range expectedValues {
		if idb.Metrics[key] != val {
			t.Error(key + " not parsed correctly. Expected: " + val + ", Got: " + idb.Metrics[key])
		}
	}
}

func TestParseTransactions(t *testing.T) {
	idb := new(InnodbStats)
	idb.Metrics = make(map[string]string)
	blob := `
Trx id counter 593258
Purge done for trx's n:o < 593256 undo n:o < 0 state: running but idle
History list length 3442
LIST OF TRANSACTIONS FOR EACH SESSION:
---TRANSACTION 593257, not started
MySQL thread id 551, OS thread handle 0x1328b8000, query id 6104496 localhost 127.0.0.1 root cleaning up
---TRANSACTION 0, not started
MySQL thread id 550, OS thread handle 0x132984000, query id 6104478 localhost root cleaning up
---TRANSACTION 0asdfasdfasdf, not started
MySQL thread id 273, OS thread handle 0x134630000, query id 6104497 localhost root init
SHOW ENGINE INNODB STATUS
---TRANSACTION 01239898273420934, not started
MySQL thread id 47, OS thread handle 0x13352c000, query id 11375 localhost root cleaning up
---TRANSACTION 100123, this is a test not started
MySQL thread id 45, OS thread handle 0x133460000, query id 11744 localhost root cleaning up
---TRANSACTION 100124, not started
MySQL thread id 44, OS thread handle 0x1334a4000, query id 11743 localhost root cleaning up
ROLLING BACK 123 lock struct(s), heap size 15, 13 row lock(s), undo log entries 1
ROLLING BACK 123 lock struct(s), heap size 15, 13 row lock(s), undo log entries 100
ROLLING BACK 123 lock struct(s), heap size 15, 13 row lock(s), undo log entries 50
ROLLING BACK 123 lock struct(s), heap size 15, 13 row lock(s), undo log entries 123
ROLLING BACK 123 lock struct(s), heap size 15, 13 row lock(s), undo log entres 123`
	idb.parseTransactions(blob)
	expectedValues := map[string]string{
		"trxes_not_started": "6",
		"undo":              "123",
	}
	for key, val := range expectedValues {
		if idb.Metrics[key] != val {
			t.Error(key + " not parsed correctly. Expected: " + val + ", Got: " + idb.Metrics[key])
		}
	}
}
