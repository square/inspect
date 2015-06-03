// Copyright (c) 2015 Square, Inc
//

package tablestat

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"sync"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/mysql/tools"
	"github.com/square/inspect/mysql/util"
	"github.com/square/inspect/os/misc"
)

const (
	innodbMetadataCheck = "SELECT @@GLOBAL.innodb_stats_on_metadata;"
	dbSizesQuery        = `
  SELECT table_schema AS db,
         SUM( data_length + index_length ) AS db_size_bytes
    FROM information_schema.TABLES
   WHERE table_schema NOT IN ('performance_schema', 'information_schema', 'mysql')
   GROUP BY 1;`
	tblSizesQuery = `
    SELECT table_schema AS db, table_name as tbl,
           data_length + index_length AS tbl_size_bytes
      FROM information_schema.TABLES
     WHERE table_schema NOT IN ('performance_schema', 'information_schema', 'mysql');`
	tblStatisticsQuery = `
SELECT table_schema AS db, table_name AS tbl, 
       rows_read, rows_changed, rows_changed_x_indexes  
  FROM INFORMATION_SCHEMA.TABLE_STATISTICS
 WHERE rows_read > 0;`
	defaultMaxConns = 5
)

// MysqlStatTables - main struct that contains connection to database, metric context, and map to database stats struct
type MysqlStatTables struct {
	util.MysqlStat
	DBs   map[string]*DBStats
	nLock *sync.Mutex
}

// DBStats represents struct that contains metrics for databases and map to tables stats struct
type DBStats struct {
	Tables  map[string]*MysqlStatPerTable
	Metrics *MysqlStatPerDB
}

// MysqlStatPerTable represents metrics for each table
type MysqlStatPerTable struct {
	SizeBytes           *metrics.Gauge
	RowsRead            *metrics.Counter
	RowsChanged         *metrics.Counter
	RowsChangedXIndexes *metrics.Counter
}

// MysqlStatPerDB - metrics for each database
type MysqlStatPerDB struct {
	SizeBytes *metrics.Gauge
}

// New initializes mysqlstat and returns it
// arguments: metrics context, username, password, path to config file for
// mysql. username and password can be left as "" if a config file is specified.
func New(m *metrics.MetricContext, user, password, host, config string) (*MysqlStatTables, error) {
	s := new(MysqlStatTables)
	s.m = m
	s.nLock = &sync.Mutex{}
	// connect to database
	var err error
	s.db, err = tools.New(user, password, host, config)
	s.nLock.Lock()
	s.DBs = make(map[string]*DBStats)
	s.nLock.Unlock()
	if err != nil { //error in connecting to database
		return nil, err
	}
	return s, nil
}

//initialize  per database metrics
func newMysqlStatPerDB(m *metrics.MetricContext, dbname string) *MysqlStatPerDB {
	o := new(MysqlStatPerDB)
	misc.InitializeMetrics(o, m, "mysqlstat."+dbname, true)
	return o
}

//initialize per table metrics
func newMysqlStatPerTable(m *metrics.MetricContext, dbname, tblname string) *MysqlStatPerTable {
	o := new(MysqlStatPerTable)
	misc.InitializeMetrics(o, m, "mysqlstat."+dbname+"."+tblname, true)
	return o
}

// Collect collects various database/table metrics
// sql.DB is thread safe so launching metrics collectors
// in their own goroutines is safe
func (s *MysqlStatTables) Collect() {
	s.wg.Add(3)
	go s.GetDBSizes()
	go s.GetTableSizes()
	go s.GetTableStatistics()
	s.wg.Wait()
}

//instantiate database metrics struct
func (s *MysqlStatTables) initializeDB(dbname string) *DBStats {
	n := new(DBStats)
	n.Metrics = newMysqlStatPerDB(s.m, dbname)
	n.Tables = make(map[string]*MysqlStatPerTable)
	return n
}

//check if database struct is instantiated, and instantiate if not
func (s *MysqlStatTables) checkDB(dbname string) {
	s.nLock.Lock()
	if _, ok := s.DBs[dbname]; !ok {
		s.DBs[dbname] = s.initializeDB(dbname)
	}
	s.nLock.Unlock()
	return
}

//check if table struct is instantiated, and instantiate if not
func (s *MysqlStatTables) checkTable(dbname, tblname string) {
	s.checkDB(dbname)
	s.nLock.Lock()
	if _, ok := s.DBs[dbname].Tables[tblname]; !ok {
		s.DBs[dbname].Tables[tblname] = newMysqlStatPerTable(s.m, dbname, tblname)
	}
	s.nLock.Unlock()
	return
}

// GetDBSizes collects information about sizes of databases
func (s *MysqlStatTables) GetDBSizes() {
	res, err := s.db.QueryReturnColumnDict(innodbMetadataCheck)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	for _, val := range res {
		if v, _ := strconv.ParseInt(string(val[0]), 10, 64); v == 1 {
			fmt.Println("Not capturing db/tbl sizes because @@GLOBAL.innodb_stats_on_metadata = 1")
			s.db.Log(errors.New("not capturing sizes: innodb_stats_on_metadata = 1"))
			s.wg.Done()
			return
		}
		break
	}

	res, err = s.db.QueryMapFirstColumnToRow(dbSizesQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	for key, value := range res {
		//key being the name of the database, value being its size in bytes
		dbname := string(key)
		size, _ := strconv.ParseInt(string(value[0]), 10, 64)
		if size > 0 {
			s.checkDB(dbname)
			s.nLock.Lock()
			s.DBs[dbname].Metrics.SizeBytes.Set(float64(size))
			s.nLock.Unlock()
		}
	}
	s.wg.Done()
	return
}

// GetTableSizes collects sizes of tables within databases
func (s *MysqlStatTables) GetTableSizes() {
	res, err := s.db.QueryReturnColumnDict(innodbMetadataCheck)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	for _, val := range res {
		if v, _ := strconv.ParseInt(string(val[0]), 10, 64); v == int64(1) {
			fmt.Println("Not capturing db/tbl sizes because @@GLOBAL.innodb_stats_on_metadata = 1")
			s.db.Log(errors.New("not capturing sizes: innodb_stats_on_metadata = 1"))
			s.wg.Done()
			return
		}
		break
	}
	res, err = s.db.QueryReturnColumnDict(tblSizesQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	tableCount := len(res["tbl"])
	for i := 0; i < tableCount; i++ {
		dbname := string(res["db"][i])
		tblname := string(res["tbl"][i])
		if res["tbl_size_bytes"][i] == "" {
			continue
		}
		s.checkDB(dbname)
		size, err := strconv.ParseInt(string(res["tbl_size_bytes"][i]), 10, 64)
		if err != nil {
			s.db.Log(err)
		}
		if size > 0 {
			s.checkTable(dbname, tblname)
			s.nLock.Lock()
			s.DBs[dbname].Tables[tblname].SizeBytes.Set(float64(size))
			s.nLock.Unlock()
		}
	}
	s.wg.Done()
	return
}

// GetTableStatistics collects table statistics: rows read, rows changed, rows changed x indices
func (s *MysqlStatTables) GetTableStatistics() {
	res, err := s.db.QueryReturnColumnDict(tblStatisticsQuery)
	if len(res) == 0 || err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	for i, tblname := range res["tbl"] {
		dbname := res["db"][i]
		rowsRead, err := strconv.ParseInt(res["rows_read"][i], 10, 64)
		if err != nil {
			s.db.Log(err)
		}
		rowsChanged, err := strconv.ParseInt(res["rows_changed"][i], 10, 64)
		if err != nil {
			s.db.Log(err)
		}
		rowsChangedXIndexes, err := strconv.ParseInt(res["rows_changed_x_indexes"][i], 10, 64)
		if err != nil {
			s.db.Log(err)
		}
		if rowsRead > 0 {
			s.checkDB(dbname)
			s.checkTable(dbname, tblname)
			s.nLock.Lock()
			s.DBs[dbname].Tables[tblname].RowsRead.Set(uint64(rowsRead))
			s.nLock.Unlock()
		}
		if rowsChanged > 0 {
			s.checkDB(dbname)
			s.checkTable(dbname, tblname)
			s.nLock.Lock()
			s.DBs[dbname].Tables[tblname].RowsChanged.Set(uint64(rowsChanged))
			s.nLock.Unlock()
		}
		if rowsChangedXIndexes > 0 {
			s.checkDB(dbname)
			s.checkTable(dbname, tblname)
			s.nLock.Lock()
			s.DBs[dbname].Tables[tblname].RowsChangedXIndexes.Set(uint64(rowsChangedXIndexes))
			s.nLock.Unlock()
		}
	}
	s.wg.Done()
	return
}

// FormatGraphite writes metrics in the form "metric_name metric_value"
// to the input writer
func (s *MysqlStatTables) FormatGraphite(w io.Writer) error {
	for dbname, db := range s.DBs {
		if !math.IsNaN(db.Metrics.SizeBytes.Get()) {
			fmt.Fprintln(w, dbname+".SizeBytes "+
				strconv.FormatFloat(db.Metrics.SizeBytes.Get(), 'f', 5, 64))
		}
		for tblname, tbl := range db.Tables {
			if !math.IsNaN(tbl.SizeBytes.Get()) {
				fmt.Fprintln(w, dbname+"."+tblname+".SizeBytes "+
					strconv.FormatFloat(tbl.SizeBytes.Get(), 'f', 5, 64))
			}
			fmt.Fprintln(w, dbname+"."+tblname+".RowsRead "+
				strconv.FormatUint(tbl.RowsRead.Get(), 10))
			fmt.Fprintln(w, dbname+"."+tblname+".RowsChanged "+
				strconv.FormatUint(tbl.RowsChanged.Get(), 10))
			fmt.Fprintln(w, dbname+"."+tblname+".RowsChangedXIndexes "+
				strconv.FormatUint(tbl.RowsChangedXIndexes.Get(), 10))
		}
	}
	return nil
}
