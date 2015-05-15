// Copyright (c) 2014 Square, Inc
//

package dbstat

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/mysql/tools"
	"github.com/square/inspect/os/misc"
)

// Collection of metrics and connection to database
type MysqlStat struct {
	Metrics *MysqlStatMetrics //collection of metrics
	m       *metrics.MetricContext
	db      tools.MysqlDB //mysql connection
	wg      sync.WaitGroup
}

// metrics being collected about the server/database
type MysqlStatMetrics struct {
	//GetSlave Stats
	SlaveSecondsBehindMaster *metrics.Gauge
	SlaveSeqFile             *metrics.Gauge
	SlavePosition            *metrics.Counter
	ReplicationRunning       *metrics.Gauge

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
	InnodbCurrentRowLocks     *metrics.Gauge
	InnodbLogOsWaits          *metrics.Gauge
	InnodbRowLockCurrentWaits *metrics.Gauge
	InnodbRowLockTimeAvg      *metrics.Gauge
	InnodbRowLockTimeMax      *metrics.Counter
	PreparedStmtCount         *metrics.Gauge
	PreparedStmtPct           *metrics.Gauge
	Queries                   *metrics.Counter
	SortMergePasses           *metrics.Counter
	ThreadsConnected          *metrics.Gauge
	Uptime                    *metrics.Counter
	ThreadsRunning            *metrics.Gauge

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
}

const (
	slaveQuery  = "SHOW SLAVE STATUS;"
	oldestQuery = `
 SELECT time FROM information_schema.processlist
  WHERE command NOT IN ('Sleep','Connect','Binlog Dump')
  ORDER BY time DESC LIMIT 1;`
	oldestTrx = `
  SELECT UNIX_TIMESTAMP(NOW()) - UNIX_TIMESTAMP(MIN(trx_started)) AS time 
    FROM information_schema.innodb_trx;`
	responseTimeQuery         = "SELECT time, count FROM INFORMATION_SCHEMA.QUERY_RESPONSE_TIME;"
	binlogQuery               = "SHOW MASTER LOGS;"
	globalStatsQuery          = "SHOW GLOBAL STATUS;"
	maxPreparedStmtCountQuery = "SHOW GLOBAL VARIABLES LIKE 'max_prepared_stmt_count';"
	longQuery                 = `
    SELECT * FROM information_schema.processlist
     WHERE command NOT IN ('Sleep', 'Connect', 'Binlog Dump')
       AND time > 30;`
	versionQuery     = "SELECT VERSION();"
	binlogStatsQuery = "SHOW MASTER STATUS;"
	stackedQuery     = `
  SELECT COUNT(*) AS identical_queries_stacked, 
         MAX(time) AS max_age, 
         GROUP_CONCAT(id SEPARATOR ' ') AS thread_ids, 
         info as query 
    FROM information_schema.processlist 
   WHERE user != 'system user'
     AND user NOT LIKE 'repl%'
     AND info IS NOT NULL
   GROUP BY 4
  HAVING COUNT(*) > 1
     AND MAX(time) > 300
   ORDER BY 2 DESC;`
	sessionQuery1 = "SELECT @@GLOBAL.max_connections;"
	sessionQuery2 = `
    SELECT IF(command LIKE 'Sleep',1,0) +
           IF(state LIKE '%master%' OR state LIKE '%slave%',1,0) AS sort_col,
           processlist.*
      FROM information_schema.processlist
     ORDER BY 1, time DESC;`
	innodbQuery      = "SHOW GLOBAL VARIABLES LIKE 'innodb_log_file_size';"
	securityQuery    = "SELECT user FROM mysql.user WHERE password = '' AND ssl_type = '';"
	slaveBackupQuery = `
SELECT COUNT(*) as count
  FROM information_schema.processlist 
 WHERE user LIKE '%backup%';`
	defaultMaxConns = 5
)

//initializes mysqlstat.
//takes as input: metrics context, username, password, path to config file for
// mysql. username and password can be left as "" if a config file is specified.
func New(m *metrics.MetricContext, user, password, host, config string) (*MysqlStat, error) {
	s := new(MysqlStat)

	// connect to database
	var err error
	s.db, err = tools.New(user, password, host, config)
	s.SetMaxConnections(defaultMaxConns)
	if err != nil {
		s.db.Log(err)
		return nil, err
	}
	s.Metrics = MysqlStatMetricsNew(m)

	return s, nil
}

// Set the max number of concurrent connections that the mysql client can use
func (s *MysqlStat) SetMaxConnections(maxConns int) {
	s.db.SetMaxConnections(maxConns)
}

//initializes metrics
func MysqlStatMetricsNew(m *metrics.MetricContext) *MysqlStatMetrics {
	c := new(MysqlStatMetrics)
	misc.InitializeMetrics(c, m, "mysqlstat", true)
	return c
}

//launches metrics collectors.
// sql.DB is safe for concurrent use by multiple goroutines
// so launching each metric collector as its own goroutine is safe
func (s *MysqlStat) Collect() {
	s.wg.Add(14)
	go s.GetVersion()
	go s.GetSlaveStats()
	go s.GetGlobalStatus()
	go s.GetBinlogStats()
	go s.GetStackedQueries()
	go s.GetSessions()
	go s.GetNumLongRunQueries()
	go s.GetQueryResponseTime()
	go s.GetBackups()
	go s.GetOldestQuery()
	go s.GetOldestTrx()
	go s.GetBinlogFiles()
	go s.GetInnodbStats()
	go s.GetSecurity()
	s.wg.Wait()
}

// get_slave_stats gets slave statistics
func (s *MysqlStat) GetSlaveStats() {
	s.Metrics.ReplicationRunning.Set(float64(-1))
	numBackups := float64(0)

	res, err := s.db.QueryReturnColumnDict(slaveBackupQuery)
	if err != nil {
		s.db.Log(err)
	} else if len(res["count"]) > 0 {
		numBackups, err = strconv.ParseFloat(string(res["count"][0]), 64)
		if err != nil {
			s.db.Log(err)
		} else {
			if numBackups > 0 {
				s.Metrics.SlaveSecondsBehindMaster.Set(float64(-1))
				s.Metrics.ReplicationRunning.Set(float64(1))
			}
		}
	}
	res, err = s.db.QueryReturnColumnDict(slaveQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}

	if (len(res["Seconds_Behind_Master"]) > 0) && (string(res["Seconds_Behind_Master"][0]) != "") {
		seconds_behind_master, err := strconv.ParseFloat(string(res["Seconds_Behind_Master"][0]), 64)
		if err != nil {
			s.db.Log(err)
			s.Metrics.SlaveSecondsBehindMaster.Set(float64(-1))
			if numBackups == 0 {
				s.Metrics.ReplicationRunning.Set(float64(-1))
			}
		} else {
			s.Metrics.SlaveSecondsBehindMaster.Set(float64(seconds_behind_master))
			s.Metrics.ReplicationRunning.Set(float64(1))
		}
	}

	relay_master_log_file, _ := res["Relay_Master_Log_File"]
	if len(relay_master_log_file) > 0 {
		tmp := strings.Split(string(relay_master_log_file[0]), ".")
		slave_seqfile, err := strconv.ParseInt(tmp[len(tmp)-1], 10, 64)
		s.Metrics.SlaveSeqFile.Set(float64(slave_seqfile))
		if err != nil {
			s.db.Log(err)
		}
	}

	if len(res["Exec_Master_Log_Pos"]) > 0 {
		slave_position, err := strconv.ParseFloat(string(res["Exec_Master_Log_Pos"][0]), 64)
		if err != nil {
			s.db.Log(err)
			s.wg.Done()
			return
		}
		s.Metrics.SlavePosition.Set(uint64(slave_position))
	}
	s.wg.Done()
	return
}

//gets global statuses
func (s *MysqlStat) GetGlobalStatus() {
	res, err := s.db.QueryReturnColumnDict(maxPreparedStmtCountQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	var max_prepared_stmt_count int64
	if err == nil && len(res["Value"]) > 0 {
		max_prepared_stmt_count, err = strconv.ParseInt(res["Value"][0], 10, 64)
		if err != nil {
			s.db.Log(err)
		}
	}

	res, err = s.db.QueryMapFirstColumnToRow(globalStatsQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	vars := map[string]interface{}{
		"Binlog_cache_disk_use":         s.Metrics.BinlogCacheDiskUse,
		"Binlog_cache_use":              s.Metrics.BinlogCacheUse,
		"Com_alter_table":               s.Metrics.ComAlterTable,
		"Com_begin":                     s.Metrics.ComBegin,
		"Com_commit":                    s.Metrics.ComCommit,
		"Com_create_table":              s.Metrics.ComCreateTable,
		"Com_delete":                    s.Metrics.ComDelete,
		"Com_delete_multi":              s.Metrics.ComDeleteMulti,
		"Com_drop_table":                s.Metrics.ComDropTable,
		"Com_insert":                    s.Metrics.ComInsert,
		"Com_insert_select":             s.Metrics.ComInsertSelect,
		"Com_replace":                   s.Metrics.ComReplace,
		"Com_replace_select":            s.Metrics.ComReplaceSelect,
		"Com_rollback":                  s.Metrics.ComRollback,
		"Com_select":                    s.Metrics.ComSelect,
		"Com_update":                    s.Metrics.ComUpdate,
		"Com_update_multi":              s.Metrics.ComUpdateMulti,
		"Created_tmp_disk_tables":       s.Metrics.CreatedTmpDiskTables,
		"Created_tmp_files":             s.Metrics.CreatedTmpFiles,
		"Created_tmp_tables":            s.Metrics.CreatedTmpTables,
		"Innodb_current_row_locks":      s.Metrics.InnodbCurrentRowLocks,
		"Innodb_log_os_waits":           s.Metrics.InnodbLogOsWaits,
		"Innodb_row_lock_current_waits": s.Metrics.InnodbRowLockCurrentWaits,
		"Innodb_row_lock_time_avg":      s.Metrics.InnodbRowLockTimeAvg,
		"Innodb_row_lock_time_max":      s.Metrics.InnodbRowLockTimeMax,
		"Prepared_stmt_count":           s.Metrics.PreparedStmtCount,
		"Queries":                       s.Metrics.Queries,
		"Sort_merge_passes":             s.Metrics.SortMergePasses,
		"Threads_connected":             s.Metrics.ThreadsConnected,
		"Uptime":                        s.Metrics.Uptime,
		"Threads_running":               s.Metrics.ThreadsRunning,
	}

	//range through expected metrics and grab from data
	for name, metric := range vars {
		v, ok := res[name]
		if ok && len(v) > 0 {
			val, err := strconv.ParseFloat(string(v[0]), 64)
			if err != nil {
				s.db.Log(err)
			}
			switch met := metric.(type) {
			case *metrics.Counter:
				met.Set(uint64(val))
			case *metrics.Gauge:
				met.Set(float64(val))
			}
		}
	}

	if max_prepared_stmt_count != 0 {
		pct := (s.Metrics.PreparedStmtCount.Get() / float64(max_prepared_stmt_count)) * 100
		s.Metrics.PreparedStmtPct.Set(pct)
	}

	s.wg.Done()
	return
}

//get time of oldest query in seconds
func (s *MysqlStat) GetOldestQuery() {
	res, err := s.db.QueryReturnColumnDict(oldestQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	t := int64(0)
	if time, ok := res["time"]; ok && len(time) > 0 {
		t, err = strconv.ParseInt(time[0], 10, 64)
		if err != nil {
			s.db.Log(err)
		}
	}
	s.Metrics.OldestQueryS.Set(float64(t))
	s.wg.Done()
	return
}

func (s *MysqlStat) GetOldestTrx() {
	res, err := s.db.QueryReturnColumnDict(oldestTrx)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	t := int64(0)
	if time, ok := res["time"]; ok && len(time) > 0 {
		t, _ = strconv.ParseInt(time[0], 10, 64)
		//only error expecting is when "NULL" is encountered
	}
	s.Metrics.OldestTrxS.Set(float64(t))
	s.wg.Done()
	return
}

//calculate query response times
func (s *MysqlStat) GetQueryResponseTime() {
	timers := map[string]*metrics.Counter{
		".000001":  s.Metrics.QueryResponseSec_000001,
		".00001":   s.Metrics.QueryResponseSec_00001,
		".0001":    s.Metrics.QueryResponseSec_0001,
		".001":     s.Metrics.QueryResponseSec_001,
		".01":      s.Metrics.QueryResponseSec_01,
		".1":       s.Metrics.QueryResponseSec_1,
		"1.":       s.Metrics.QueryResponseSec1_,
		"10.":      s.Metrics.QueryResponseSec10_,
		"100.":     s.Metrics.QueryResponseSec100_,
		"1000.":    s.Metrics.QueryResponseSec1000_,
		"10000.":   s.Metrics.QueryResponseSec10000_,
		"100000.":  s.Metrics.QueryResponseSec100000_,
		"1000000.": s.Metrics.QueryResponseSec100000_,
	}

	res, err := s.db.QueryReturnColumnDict(responseTimeQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}

	for i, time := range res["time"] {
		count, err := strconv.ParseInt(res["count"][i], 10, 64)
		if err != nil {
			s.db.Log(err)
		}
		if count < 1 {
			continue
		}
		key := strings.Trim(time, " 0")
		if timer, ok := timers[key]; ok {
			timer.Set(uint64(count))
		}
	}
	s.wg.Done()
	return
}

//gets status on binary logs
func (s *MysqlStat) GetBinlogFiles() {
	res, err := s.db.QueryReturnColumnDict(binlogQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	s.Metrics.BinlogFiles.Set(float64(len(res["File_size"])))
	binlog_total_size := int64(0)
	for _, size := range res["File_size"] {
		si, err := strconv.ParseInt(size, 10, 64)
		if err != nil {
			s.db.Log(err) //don't return err so we can continue with more values
		}
		binlog_total_size += si
	}
	s.Metrics.BinlogSize.Set(float64(binlog_total_size))
	s.wg.Done()
	return
}

//get number of long running queries
func (s *MysqlStat) GetNumLongRunQueries() {
	res, err := s.db.QueryReturnColumnDict(longQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	found_sql := len(res["ID"])
	s.Metrics.ActiveLongRunQueries.Set(float64(found_sql))
	s.wg.Done()
	return
}

//get version
//version is of the form '1.2.34-56.7' or '9.8.76a-54.3-log'
// want to represent version in form '1.234567' or '9.876543'
func (s *MysqlStat) GetVersion() {
	res, err := s.db.QueryReturnColumnDict(versionQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	if len(res["VERSION()"]) == 0 {
		s.wg.Done()
		return
	}
	version := res["VERSION()"][0]
	//filter out letters
	f := func(r rune) bool {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return true
		}
		return false
	}
	version = strings.Join(strings.FieldsFunc(version, f), "")                      //filters out letters from string
	version = strings.Replace(strings.Replace(version, "-", ".", -1), "_", ".", -1) //replaces "_" and "-" with "."
	leading := float64(len(strings.Split(version, ".")[0]))
	version = strings.Replace(version, ".", "", -1)
	ver, err := strconv.ParseFloat(version, 64)
	ver /= math.Pow(10.0, (float64(len(version)) - leading))
	s.Metrics.Version.Set(ver)
	if err != nil {
		s.db.Log(err)
	}
	s.wg.Done()
	return
}

// get binlog statistics
func (s *MysqlStat) GetBinlogStats() {
	res, err := s.db.QueryReturnColumnDict(binlogStatsQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	if len(res["File"]) == 0 || len(res["Position"]) == 0 {
		s.wg.Done()
		return
	}

	v, err := strconv.ParseFloat(strings.Split(string(res["File"][0]), ".")[1], 64)
	if err != nil {
		s.db.Log(err)
	}
	s.Metrics.BinlogSeqFile.Set(float64(v))
	v, err = strconv.ParseFloat(string(res["Position"][0]), 64)
	if err != nil {
		s.db.Log(err)
	}
	s.Metrics.BinlogPosition.Set(uint64(v))
	s.wg.Done()
	return
}

//detect application bugs which result in multiple instance of the same query "stacking up"/ executing at the same time
func (s *MysqlStat) GetStackedQueries() {
	cmd := stackedQuery
	res, err := s.db.QueryReturnColumnDict(cmd)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	if len(res["identical_queries_stacked"]) > 0 {
		count, err := strconv.ParseFloat(string(res["identical_queries_stacked"][0]), 64)
		if err != nil {
			s.db.Log(err)
		}
		s.Metrics.IdenticalQueriesStacked.Set(float64(count))
		age, err := strconv.ParseFloat(string(res["max_age"][0]), 64)
		if err != nil {
			s.db.Log(err)
		}
		s.Metrics.IdenticalQueriesMaxAge.Set(float64(age))
	}
	s.wg.Done()
	return
}

//get session stats
func (s *MysqlStat) GetSessions() {
	res, err := s.db.QueryReturnColumnDict(sessionQuery1)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	var max_sessions int64
	for _, val := range res {
		max_sessions, err = strconv.ParseInt(val[0], 10, 64)
		if err != nil {
			s.db.Log(err)
		}
		s.Metrics.MaxConnections.Set(float64(max_sessions))
	}
	res, err = s.db.QueryReturnColumnDict(sessionQuery2)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	if len(res["COMMAND"]) == 0 {
		s.wg.Done()
		return
	}
	current_total := len(res["COMMAND"])
	s.Metrics.CurrentSessions.Set(float64(current_total))
	pct := (float64(current_total) / float64(max_sessions)) * 100
	s.Metrics.CurrentConnectionsPct.Set(pct)

	active := 0.0
	unauthenticated := 0
	locked := 0
	table_lock_wait := 0
	global_read_lock_wait := 0
	copy_to_table := 0
	statistics := 0
	for i, val := range res["COMMAND"] {
		if val != "Sleep" && val != "Connect" && val != "Binlog Dump" {
			active += 1
		}
		if matched, err := regexp.MatchString("unauthenticated", res["USER"][i]); err == nil && matched {
			unauthenticated += 1
		}
		if matched, err := regexp.MatchString("Locked", res["STATE"][i]); err == nil && matched {
			locked += 1
		} else if matched, err := regexp.MatchString("Table Lock", res["STATE"][i]); err == nil && matched {
			table_lock_wait += 1
		} else if matched, err := regexp.MatchString("Waiting for global read lock", res["STATE"][i]); err == nil && matched {
			global_read_lock_wait += 1
		} else if matched, err := regexp.MatchString("opy.*table", res["STATE"][i]); err == nil && matched {
			copy_to_table += 1
		} else if matched, err := regexp.MatchString("statistics", res["STATE"][i]); err == nil && matched {
			statistics += 1
		}
	}
	s.Metrics.ActiveSessions.Set(active)
	s.Metrics.BusySessionPct.Set((active / float64(current_total)) * float64(100))
	s.Metrics.UnauthenticatedSessions.Set(float64(unauthenticated))
	s.Metrics.LockedSessions.Set(float64(locked))
	s.Metrics.SessionTablesLocks.Set(float64(table_lock_wait))
	s.Metrics.SessionGlobalReadLocks.Set(float64(global_read_lock_wait))
	s.Metrics.SessionsCopyingToTable.Set(float64(copy_to_table))
	s.Metrics.SessionsStatistics.Set(float64(statistics))

	s.wg.Done()
	return
}

//metrics from innodb
func (s *MysqlStat) GetInnodbStats() {
	res, err := s.db.QueryReturnColumnDict(innodbQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	var innodb_log_file_size int64
	if err == nil && len(res["Value"]) > 0 {
		innodb_log_file_size, err = strconv.ParseInt(res["Value"][0], 10, 64)
		if err != nil {
			s.db.Log(err)
		}
	}

	res, err = s.db.QueryReturnColumnDict("SHOW ENGINE INNODB STATUS")
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}

	//parse the result
	var idb *tools.InnodbStats
	idb, _ = tools.ParseInnodbStats(res["Status"][0])
	vars := map[string]interface{}{
		"OS_file_reads":               s.Metrics.OSFileReads,
		"OS_file_writes":              s.Metrics.OSFileWrites,
		"adaptive_hash":               s.Metrics.AdaptiveHash,
		"avg_bytes_per_read":          s.Metrics.AvgBytesPerRead,
		"buffer_pool_hit_rate":        s.Metrics.BufferPoolHitRate,
		"buffer_pool_size":            s.Metrics.BufferPoolSize,
		"cache_hit_pct":               s.Metrics.CacheHitPct,
		"checkpoint_age":              s.Metrics.InnodbCheckpointAge,
		"checkpoint_age_target":       s.Metrics.InnodbCheckpointAgeTarget,
		"database_pages":              s.Metrics.DatabasePages,
		"dictionary_cache":            s.Metrics.DictionaryCache,
		"dictionary_memory_allocated": s.Metrics.DictionaryMemoryAllocated,
		"file_system":                 s.Metrics.FileSystem,
		"free_buffers":                s.Metrics.FreeBuffers,
		"fsyncs_per_s":                s.Metrics.FsyncsPerSec,
		"history_list":                s.Metrics.InnodbHistoryLinkList,
		"last_checkpoint_at":          s.Metrics.InnodbLastCheckpointAt,
		"lock_system":                 s.Metrics.LockSystem,
		"log_flushed_up_to":           s.Metrics.InnodbLogFlushedUpTo,
		"log_io_per_sec":              s.Metrics.LogIOPerSec,
		"log_sequence_number":         s.Metrics.InnodbLogSequenceNumber,
		"max_checkpoint_age":          s.Metrics.InnodbMaxCheckpointAge,
		"modified_age":                s.Metrics.InnodbModifiedAge,
		"modified_db_pages":           s.Metrics.ModifiedDBPages,
		"old_database_pages":          s.Metrics.OldDatabasePages,
		"page_hash":                   s.Metrics.PageHash,
		"pages_flushed_up_to":         s.Metrics.PagesFlushedUpTo,
		"pages_made_young":            s.Metrics.PagesMadeYoung,
		"pages_read":                  s.Metrics.PagesRead,
		"pending_chkp_writes":         s.Metrics.InnodbPendingCheckpointWrites,
		"pending_log_writes":          s.Metrics.InnodbPendingLogWrites,
		"pending_reads":               s.Metrics.PendingReads,
		"pending_writes_lru":          s.Metrics.PendingWritesLRU,
		"reads_per_s":                 s.Metrics.ReadsPerSec,
		"recovery_system":             s.Metrics.RecoverySystem,
		"total_mem":                   s.Metrics.TotalMem,
		"total_mem_by_read_views":     s.Metrics.TotalMemByReadViews,
		"trx_id":                      s.Metrics.TransactionID,
		"trxes_not_started":           s.Metrics.InnodbTransactionsNotStarted,
		"undo":                        s.Metrics.InnodbUndo,
		"writes_per_s":                s.Metrics.WritesPerSec,
	}
	//store the result in the appropriate metrics
	for name, metric := range vars {
		v, ok := idb.Metrics[name]
		if ok {
			val, err := strconv.ParseFloat(string(v), 64)
			if err != nil {
				s.db.Log(err)
			}
			//case based on type so can switch between Gauge and Counter easily
			switch met := metric.(type) {
			case *metrics.Counter:
				met.Set(uint64(val))
			case *metrics.Gauge:
				met.Set(float64(val))
			}
		}
	}
	if lsn, ok := idb.Metrics["log_sequence_number"]; ok && innodb_log_file_size != 0 {
		lsn_s, _ := strconv.ParseFloat(lsn, 64)
		s.Metrics.InnodbLogWriteRatio.Set((lsn_s * 3600.0) / float64(innodb_log_file_size))
	}
	s.wg.Done()
	return
}

//get backups count
func (s *MysqlStat) GetBackups() {
	out, err := exec.Command("ps", "aux").Output()
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	blob := string(out)
	lines := strings.Split(blob, "\n")
	backupProcs := 0
	for _, line := range lines {
		words := strings.Split(line, " ")
		if len(words) < 10 {
			continue
		}
		command := strings.Join(words[10:], " ")
		if strings.Contains(command, "innobackupex") ||
			strings.Contains(command, "mysqldump") ||
			strings.Contains(command, "mydumper") {
			backupProcs += 1
		}
	}
	s.Metrics.BackupsRunning.Set(float64(backupProcs))
	s.wg.Done()
	return
}

//get count unsecure users
func (s *MysqlStat) GetSecurity() {
	res, err := s.db.QueryReturnColumnDict(securityQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	s.Metrics.UnsecureUsers.Set(float64(len(res["users"])))
	s.wg.Done()
	return
}

// Closes database connection
func (s *MysqlStat) Close() {
	s.db.Close()
}

//CallByMethodName searches for a method implemented
// by s with name. Runs all methods that match names.
func (s *MysqlStat) CallByMethodName(name string) error {
	r := reflect.TypeOf(s)
	re := regexp.MustCompile(strings.ToLower(name))
	f := false
	for i := 0; i < r.NumMethod(); i++ {
		n := strings.ToLower(r.Method(i).Name)
		if strings.Contains(n, "get") && re.MatchString(n) {
			s.wg.Add(1)
			reflect.ValueOf(s).Method(i).Call([]reflect.Value{})
			f = true
		}
	}
	if !f {
		return errors.New("Could not find function")
	}
	return nil
}

//returns []string of metric values of the form:
// "metric_name metric_value"
// This is the form that stats-collector uses to send messages to graphite
func (s *MysqlStat) FormatGraphite(w io.Writer) error {
	metricstype := reflect.TypeOf(*s.Metrics)
	metricvalue := reflect.ValueOf(*s.Metrics)
	for i := 0; i < metricvalue.NumField(); i++ {
		n := metricvalue.Field(i).Interface()
		name := metricstype.Field(i).Name
		switch metric := n.(type) {
		case *metrics.Counter:
			if !math.IsNaN(metric.ComputeRate()) {
				fmt.Fprintln(w, name+".Value "+strconv.FormatUint(metric.Get(), 10))
				fmt.Fprintln(w, name+".Rate "+strconv.FormatFloat(metric.ComputeRate(),
					'f', 5, 64))
			}
		case *metrics.Gauge:
			if !math.IsNaN(metric.Get()) {
				fmt.Fprintln(w, name+".Value "+strconv.FormatFloat(metric.Get(), 'f', 5, 64))
			}
		}
	}
	return nil
}
