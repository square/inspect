// Copyright (c) 2014 Square, Inc
//

package dbstat

import (
	"fmt"
	"io"
	"math"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/mysql/qrt"
	"github.com/square/inspect/mysql/tools"
	"github.com/square/inspect/mysql/util"
	"github.com/square/inspect/os/misc"
)

// MysqlStatDBs represents collection of metrics and connection to database
type MysqlStatDBs struct {
	util.MysqlStat
	Metrics        *MysqlStatMetrics //collection of metrics
	MasterHostname string
}

// MysqlStatMetrics represents metrics being collected about the server/database
type MysqlStatMetrics struct {
	//GetSlave Stats
	SlaveSecondsBehindMaster *metrics.Gauge
	SlaveSeqFile             *metrics.Gauge
	SlavePosition            *metrics.Counter
	ReplicationRunning       *metrics.Gauge
	RelayLogSpace            *metrics.Gauge

	//GetGlobalStatus
	AbortedConnects      *metrics.Counter
	BinlogCacheDiskUse   *metrics.Counter
	BinlogCacheUse       *metrics.Counter
	ComAlterTable        *metrics.Counter
	ComBegin             *metrics.Counter
	ComCommit            *metrics.Counter
	ComCreateTable       *metrics.Counter
	ComDelete            *metrics.Counter
	ComDeleteMulti       *metrics.Counter
	ComDropTable         *metrics.Counter
	ComInsert            *metrics.Counter
	ComInsertSelect      *metrics.Counter
	ComReplace           *metrics.Counter
	ComReplaceSelect     *metrics.Counter
	ComRollback          *metrics.Counter
	ComSelect            *metrics.Counter
	ComUpdate            *metrics.Counter
	ComUpdateMulti       *metrics.Counter
	CreatedTmpDiskTables *metrics.Counter
	CreatedTmpFiles      *metrics.Counter
	CreatedTmpTables     *metrics.Counter
	InnodbLogOsWaits     *metrics.Gauge
	InnodbRowLockWaits   *metrics.Counter
	InnodbRowLockTimeAvg *metrics.Gauge
	InnodbRowLockTimeMax *metrics.Counter
	PreparedStmtCount    *metrics.Gauge
	PreparedStmtPct      *metrics.Gauge
	Queries              *metrics.Counter
	SortMergePasses      *metrics.Counter
	ThreadsConnected     *metrics.Gauge
	Uptime               *metrics.Counter
	ThreadsRunning       *metrics.Gauge

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
	QueryResponsePctl50  *metrics.Gauge
	QueryResponsePctl75  *metrics.Gauge
	QueryResponsePctl90  *metrics.Gauge
	QueryResponsePctl95  *metrics.Gauge
	QueryResponsePctl99  *metrics.Gauge
	QueryResponsePctl999 *metrics.Gauge

	//GetSSL
	HasSSL *metrics.Gauge

	//GetReadOnly
	IsReadOnly      *metrics.Gauge
	IsSuperReadOnly *metrics.Gauge
}

const (
	slaveQuery  = "SHOW SLAVE STATUS;"
	oldestQuery = `
 SELECT time FROM information_schema.processlist
  WHERE command NOT IN ('Sleep','Connect','Binlog Dump','Binlog Dump GTID')
  ORDER BY time DESC LIMIT 1;`
	oldestTrx = `
  SELECT UNIX_TIMESTAMP(NOW()) - UNIX_TIMESTAMP(MIN(trx_started)) AS time
    FROM information_schema.innodb_trx;`
	responseTimeQuery         = "SELECT time, count, total FROM INFORMATION_SCHEMA.QUERY_RESPONSE_TIME WHERE TIME!='TOO LONG';"
	binlogQuery               = "SHOW MASTER LOGS;"
	globalStatsQuery          = "SHOW GLOBAL STATUS;"
	maxPreparedStmtCountQuery = "SHOW GLOBAL VARIABLES LIKE 'max_prepared_stmt_count';"
	longQuery                 = `
    SELECT * FROM information_schema.processlist
     WHERE command NOT IN ('Sleep', 'Connect', 'Binlog Dump','Binlog Dump GTID')
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
	securityQuery    = "SELECT COUNT(*) FROM mysql.user WHERE (password = '' OR password IS NULL) AND (x509_subject='' OR x509_subject IS NULL);"
	slaveBackupQuery = `
SELECT COUNT(*) as count
  FROM information_schema.processlist
 WHERE user LIKE '%backup%';`
	sslQuery           = "SELECT @@have_ssl;"
	defaultMaxConns    = 5
	readOnlyQuery      = "SELECT @@read_only;"
	superReadOnlyQuery = "SELECT @@super_read_only;"
	autoincQuery       = `SELECT * FROM (select
    table_schema,
    table_name,
    column_name,
    proper_type,
    auto_increment,
    max_size,
    (((max_size - auto_increment) / max_size ) * 100) AS pct_diff
  from
    INFORMATION_SCHEMA.columns
    natural join INFORMATION_SCHEMA.tables
    join (
      select 'tinyint' as proper_type, 127 as max_size
      union all
      select 'tinyint unsigned' as proper_type, 255 as max_size
      union all
      select 'smallint' as proper_type, 32767 as max_size
      union all
      select 'smallint unsigned' as proper_type, 65535 as max_size
      union all
      select 'mediumint' as proper_type, 8388607 as max_size
      union all
      select 'mediumint unsigned' as proper_type, 16777215 as max_size
      union all
      select 'int' as proper_type, 2147483647 as max_size
      union all
      select 'int unsigned' as proper_type, 4294967295 as max_size
      union all
      select 'bigint' as proper_type, 9223372036854775807 as max_size
      union all
      select 'bigint unsigned' as proper_type, 18446744073709551615 as max_size
    ) maxes ON (proper_type = CONCAT(LEFT(column_type, GREATEST(0, LOCATE('(', column_type)-1)), RIGHT(column_type, LENGTH(column_type)-LOCATE(')', column_type))))
  where
    table_schema NOT IN ('common_schema', 'mysql', '_pending_drops')
    AND extra like '%auto_increment%') AS a
  WHERE pct_diff < 40;`
)

// New initializes mysqlstat
// arguments: metrics context, username, password, path to config file for
// mysql. username and password can be left as "" if a config file is specified.
func New(m *metrics.MetricContext, user, password, host, config string) (*MysqlStatDBs, error) {
	s := new(MysqlStatDBs)
	// connect to database
	var err error
	s.Db, err = tools.New(user, password, host, config)
	if err != nil { //error in connecting to database
		s.Db.Log(err)
		return nil, err
	}
	s.SetMaxConnections(defaultMaxConns)
	s.Metrics = MysqlStatMetricsNew(m)
	return s, nil
}

// MysqlStatMetricsNew initializes metrics and registers with metriccontext
func MysqlStatMetricsNew(m *metrics.MetricContext) *MysqlStatMetrics {
	c := new(MysqlStatMetrics)
	misc.InitializeMetrics(c, m, "mysqlstat", true)
	return c
}

// Collect launches metrics collectors.
// sql.DB is safe for concurrent use by multiple goroutines
// so launching each metric collector as its own goroutine is safe
func (s *MysqlStatDBs) Collect() {

	s.GetVersion()

	var queryFuncList = []func(){
		s.GetSlaveStats,
		s.GetGlobalStatus,
		s.GetBinlogStats,
		s.GetStackedQueries,
		s.GetSessions,
		s.GetNumLongRunQueries,
		s.GetQueryResponseTime,
		s.GetBackups,
		s.GetOldestQuery,
		s.GetOldestTrx,
		s.GetBinlogFiles,
		s.GetInnodbStats,
		s.GetSecurity,
		s.GetSSL,
		s.GetReadOnly,
	}
	util.CollectInParallel(queryFuncList)
}

// GetSlaveStats returns statistics regarding mysql replication
func (s *MysqlStatDBs) GetSlaveStats() {
	s.Metrics.ReplicationRunning.Set(float64(-1))
	numBackups := float64(0)

	res, err := s.Db.QueryReturnColumnDict(slaveBackupQuery)
	if err != nil {
		s.Db.Log(err)
	} else if len(res["count"]) > 0 {
		numBackups, err = strconv.ParseFloat(string(res["count"][0]), 64)
		if err != nil {
			s.Db.Log(err)
		} else {
			if numBackups > 0 {
				s.Metrics.SlaveSecondsBehindMaster.Set(float64(-1))
				s.Metrics.ReplicationRunning.Set(float64(1))
			}
		}
	}
	res, err = s.Db.QueryReturnColumnDict(slaveQuery)
	if err != nil {
		s.Db.Log(err)
		return
	}

	if len(res["Master_Host"]) > 0 {
		s.MasterHostname = string(res["Master_Host"][0])
	}

	if (len(res["Seconds_Behind_Master"]) > 0) && (string(res["Seconds_Behind_Master"][0]) != "") {
		secondsBehindMaster, err := strconv.ParseFloat(string(res["Seconds_Behind_Master"][0]), 64)
		if err != nil {
			s.Db.Log(err)
			s.Metrics.SlaveSecondsBehindMaster.Set(float64(-1))
			if numBackups == 0 {
				s.Metrics.ReplicationRunning.Set(float64(-1))
			}
		} else {
			s.Metrics.SlaveSecondsBehindMaster.Set(float64(secondsBehindMaster))
			s.Metrics.ReplicationRunning.Set(float64(1))
		}
	}

	relayMasterLogFile, _ := res["Relay_Master_Log_File"]
	if len(relayMasterLogFile) > 0 {
		tmp := strings.Split(string(relayMasterLogFile[0]), ".")
		slaveSeqFile, err := strconv.ParseInt(tmp[len(tmp)-1], 10, 64)
		if err != nil {
			s.Db.Log(err)
		}
		s.Metrics.SlaveSeqFile.Set(float64(slaveSeqFile))
	}

	if len(res["Exec_Master_Log_Pos"]) > 0 {
		slavePosition, err := strconv.ParseFloat(string(res["Exec_Master_Log_Pos"][0]), 64)
		if err != nil {
			s.Db.Log(err)
			return
		}
		s.Metrics.SlavePosition.Set(uint64(slavePosition))
	}

	if (len(res["Relay_Log_Space"]) > 0) && (string(res["Relay_Log_Space"][0]) != "") {
		relay_log_space, err := strconv.ParseFloat(string(res["Relay_Log_Space"][0]), 64)
		if err != nil {
			s.Db.Log(err)
		} else {
			s.Metrics.RelayLogSpace.Set(float64(relay_log_space))
		}
	}
	return
}

// GetGlobalStatus collects information returned by global status
func (s *MysqlStatDBs) GetGlobalStatus() {
	res, err := s.Db.QueryReturnColumnDict(maxPreparedStmtCountQuery)
	if err != nil {
		s.Db.Log(err)
		return
	}
	var maxPreparedStmtCount int64
	if err == nil && len(res["Value"]) > 0 {
		maxPreparedStmtCount, err = strconv.ParseInt(res["Value"][0], 10, 64)
		if err != nil {
			s.Db.Log(err)
		}
	}

	res, err = s.Db.QueryMapFirstColumnToRow(globalStatsQuery)
	if err != nil {
		s.Db.Log(err)
		return
	}
	vars := map[string]interface{}{
		"Aborted_connects":         s.Metrics.AbortedConnects,
		"Binlog_cache_disk_use":    s.Metrics.BinlogCacheDiskUse,
		"Binlog_cache_use":         s.Metrics.BinlogCacheUse,
		"Com_alter_table":          s.Metrics.ComAlterTable,
		"Com_begin":                s.Metrics.ComBegin,
		"Com_commit":               s.Metrics.ComCommit,
		"Com_create_table":         s.Metrics.ComCreateTable,
		"Com_delete":               s.Metrics.ComDelete,
		"Com_delete_multi":         s.Metrics.ComDeleteMulti,
		"Com_drop_table":           s.Metrics.ComDropTable,
		"Com_insert":               s.Metrics.ComInsert,
		"Com_insert_select":        s.Metrics.ComInsertSelect,
		"Com_replace":              s.Metrics.ComReplace,
		"Com_replace_select":       s.Metrics.ComReplaceSelect,
		"Com_rollback":             s.Metrics.ComRollback,
		"Com_select":               s.Metrics.ComSelect,
		"Com_update":               s.Metrics.ComUpdate,
		"Com_update_multi":         s.Metrics.ComUpdateMulti,
		"Created_tmp_disk_tables":  s.Metrics.CreatedTmpDiskTables,
		"Created_tmp_files":        s.Metrics.CreatedTmpFiles,
		"Created_tmp_tables":       s.Metrics.CreatedTmpTables,
		"Innodb_log_os_waits":      s.Metrics.InnodbLogOsWaits,
		"Innodb_row_lock_waits":    s.Metrics.InnodbRowLockWaits,
		"Innodb_row_lock_time_avg": s.Metrics.InnodbRowLockTimeAvg,
		"Innodb_row_lock_time_max": s.Metrics.InnodbRowLockTimeMax,
		"Prepared_stmt_count":      s.Metrics.PreparedStmtCount,
		"Queries":                  s.Metrics.Queries,
		"Sort_merge_passes":        s.Metrics.SortMergePasses,
		"Threads_connected":        s.Metrics.ThreadsConnected,
		"Uptime":                   s.Metrics.Uptime,
		"Threads_running":          s.Metrics.ThreadsRunning,
	}

	//range through expected metrics and grab from data
	for name, metric := range vars {
		v, ok := res[name]
		if ok && len(v) > 0 {
			val, err := strconv.ParseFloat(string(v[0]), 64)
			if err != nil {
				s.Db.Log(err)
			}
			switch met := metric.(type) {
			case *metrics.Counter:
				met.Set(uint64(val))
			case *metrics.Gauge:
				met.Set(float64(val))
			}
		}
	}

	if maxPreparedStmtCount != 0 {
		pct := (s.Metrics.PreparedStmtCount.Get() / float64(maxPreparedStmtCount)) * 100
		s.Metrics.PreparedStmtPct.Set(pct)
	}

	return
}

// GetOldestQuery collects the time of oldest query in seconds
func (s *MysqlStatDBs) GetOldestQuery() {
	res, err := s.Db.QueryReturnColumnDict(oldestQuery)
	if err != nil {
		s.Db.Log(err)
		return
	}
	t := int64(0)
	if time, ok := res["time"]; ok && len(time) > 0 {
		t, err = strconv.ParseInt(time[0], 10, 64)
		if err != nil {
			s.Db.Log(err)
		}
	}
	s.Metrics.OldestQueryS.Set(float64(t))
	return
}

// GetOldestTrx collects information about oldest transaction
func (s *MysqlStatDBs) GetOldestTrx() {
	res, err := s.Db.QueryReturnColumnDict(oldestTrx)
	if err != nil {
		s.Db.Log(err)
		return
	}
	t := int64(0)
	if time, ok := res["time"]; ok && len(time) > 0 {
		t, _ = strconv.ParseInt(time[0], 10, 64)
		//only error expecting is when "NULL" is encountered
	}
	s.Metrics.OldestTrxS.Set(float64(t))
	return
}

// FlushQueryResponseTime flushes the Response Time Histogram
func (s *MysqlStatDBs) FlushQueryResponseTime() error {
	var flushquery string
	version := strconv.FormatFloat(s.Metrics.Version.Get(), 'f', -1, 64)[0:3]

	switch {
	case version == "5.6":
		flushquery = "SET GLOBAL query_response_time_flush=1"
	case version == "5.5":
		flushquery = "FLUSH NO_WRITE_TO_BINLOG QUERY_RESPONSE_TIME"
	default:
		err := fmt.Errorf("Version unsupported: %s", version)
		return err
	}

	err := s.Db.DbExec(flushquery)
	if err != nil {
		s.Db.Log(err)
		return err
	}

	return nil
}

// GetQueryResponseTime collects various query response times
func (s *MysqlStatDBs) GetQueryResponseTime() {
	var h qrt.MysqlQrtHistogram
	// percentiles to retrieve
	p := [6]float64{.5, .75, .90, .95, .99, .999}

	pctls := map[float64]*metrics.Gauge{
		.50:  s.Metrics.QueryResponsePctl50,
		.75:  s.Metrics.QueryResponsePctl75,
		.90:  s.Metrics.QueryResponsePctl90,
		.95:  s.Metrics.QueryResponsePctl95,
		.99:  s.Metrics.QueryResponsePctl99,
		.999: s.Metrics.QueryResponsePctl999,
	}

	res, err := s.Db.QueryReturnColumnDict(responseTimeQuery)
	if err != nil {
		s.Db.Log(err)
		return
	}

	s.FlushQueryResponseTime()

	for i := 0; i < len(res["time"]); i++ {
		// time and total are varchars in I_S.Query_Response_Time
		time, err := strconv.ParseFloat(strings.TrimSpace(res["time"][i]), 64)
		if err != nil {
			s.Db.Log(err)
		}

		count, err := strconv.ParseInt(res["count"][i], 10, 64)
		if err != nil {
			s.Db.Log(err)
		}

		total, err := strconv.ParseFloat(strings.TrimSpace(res["total"][i]), 64)
		if err != nil {
			s.Db.Log(err)
		}

		h = append(h, qrt.NewMysqlQrtBucket(time, count, total))
	}

	for _, x := range p {
		pctls[x].Set(h.Percentile(x) * 1000) // QRT Is in s and we want to display in ms.
	}

	return
}

// GetBinlogFiles collects status on binary logs
func (s *MysqlStatDBs) GetBinlogFiles() {
	res, err := s.Db.QueryReturnColumnDict(binlogQuery)
	if err != nil {
		s.Db.Log(err)
		return
	}
	s.Metrics.BinlogFiles.Set(float64(len(res["File_size"])))
	binlogTotalSize := int64(0)
	for _, size := range res["File_size"] {
		si, err := strconv.ParseInt(size, 10, 64)
		if err != nil {
			s.Db.Log(err) //don't return err so we can continue with more values
		}
		binlogTotalSize += si
	}
	s.Metrics.BinlogSize.Set(float64(binlogTotalSize))
	return
}

// GetNumLongRunQueries collects number of long running queries
func (s *MysqlStatDBs) GetNumLongRunQueries() {
	res, err := s.Db.QueryReturnColumnDict(longQuery)
	if err != nil {
		s.Db.Log(err)
		return
	}
	foundSql := len(res["ID"])
	s.Metrics.ActiveLongRunQueries.Set(float64(foundSql))
	return
}

// GetVersion collects version information about current instance
// version is of the form '1.2.34-56.7' or '9.8.76a-54.3-log'
// want to represent version in form '1.234567' or '9.876543'
func (s *MysqlStatDBs) GetVersion() {
	res, err := s.Db.QueryReturnColumnDict(versionQuery)
	if err != nil {
		s.Db.Log(err)
		return
	}
	if len(res["VERSION()"]) == 0 {
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
	if err != nil {
		s.Db.Log(err)
	}
	ver /= math.Pow(10.0, (float64(len(version)) - leading))
	s.Metrics.Version.Set(ver)
	return
}

// GetBinlogStats collect statistics about binlog (position etc)
func (s *MysqlStatDBs) GetBinlogStats() {
	res, err := s.Db.QueryReturnColumnDict(binlogStatsQuery)
	if err != nil {
		s.Db.Log(err)
		return
	}
	if len(res["File"]) == 0 || len(res["Position"]) == 0 {
		return
	}

	v, err := strconv.ParseFloat(strings.Split(string(res["File"][0]), ".")[1], 64)
	if err != nil {
		s.Db.Log(err)
	}
	s.Metrics.BinlogSeqFile.Set(float64(v))
	v, err = strconv.ParseFloat(string(res["Position"][0]), 64)
	if err != nil {
		s.Db.Log(err)
	}
	s.Metrics.BinlogPosition.Set(uint64(v))
	return
}

// GetStackedQueries collects information about stacked queries. It can be
// used to detect application bugs which result in multiple instance of the same
// query "stacking up"/ executing at the same time
func (s *MysqlStatDBs) GetStackedQueries() {
	cmd := stackedQuery
	res, err := s.Db.QueryReturnColumnDict(cmd)
	if err != nil {
		s.Db.Log(err)
		return
	}
	if len(res["identical_queries_stacked"]) > 0 {
		count, err := strconv.ParseFloat(string(res["identical_queries_stacked"][0]), 64)
		if err != nil {
			s.Db.Log(err)
		}
		s.Metrics.IdenticalQueriesStacked.Set(float64(count))
		age, err := strconv.ParseFloat(string(res["max_age"][0]), 64)
		if err != nil {
			s.Db.Log(err)
		}
		s.Metrics.IdenticalQueriesMaxAge.Set(float64(age))
	}
	return
}

// GetSessions collects statistics about sessions
func (s *MysqlStatDBs) GetSessions() {
	res, err := s.Db.QueryReturnColumnDict(sessionQuery1)
	if err != nil {
		s.Db.Log(err)
		return
	}
	var maxSessions int64
	for _, val := range res {
		maxSessions, err = strconv.ParseInt(val[0], 10, 64)
		if err != nil {
			s.Db.Log(err)
		}
		s.Metrics.MaxConnections.Set(float64(maxSessions))
	}
	res, err = s.Db.QueryReturnColumnDict(sessionQuery2)
	if err != nil {
		s.Db.Log(err)
		return
	}
	if len(res["COMMAND"]) == 0 {
		return
	}
	currentTotal := len(res["COMMAND"])
	s.Metrics.CurrentSessions.Set(float64(currentTotal))
	pct := (float64(currentTotal) / float64(maxSessions)) * 100
	s.Metrics.CurrentConnectionsPct.Set(pct)

	active := 0.0
	unauthenticated := 0
	locked := 0
	tableLockWait := 0
	globalReadLockWait := 0
	copyToTable := 0
	statistics := 0
	for i, val := range res["COMMAND"] {
		if val != "Sleep" && val != "Connect" && val != "Binlog Dump" && val != "Binlog Dump GTID" {
			active++
		}
		if matched, err := regexp.MatchString("unauthenticated", res["USER"][i]); err == nil && matched {
			unauthenticated++
		}
		if matched, err := regexp.MatchString("Locked", res["STATE"][i]); err == nil && matched {
			locked++
		} else if matched, err := regexp.MatchString("Table Lock", res["STATE"][i]); err == nil && matched {
			tableLockWait++
		} else if matched, err := regexp.MatchString("Waiting for global read lock", res["STATE"][i]); err == nil && matched {
			globalReadLockWait++
		} else if matched, err := regexp.MatchString("opy.*table", res["STATE"][i]); err == nil && matched {
			copyToTable++
		} else if matched, err := regexp.MatchString("statistics", res["STATE"][i]); err == nil && matched {
			statistics++
		}
	}
	s.Metrics.ActiveSessions.Set(active)
	s.Metrics.BusySessionPct.Set((active / float64(currentTotal)) * float64(100))
	s.Metrics.UnauthenticatedSessions.Set(float64(unauthenticated))
	s.Metrics.LockedSessions.Set(float64(locked))
	s.Metrics.SessionTablesLocks.Set(float64(tableLockWait))
	s.Metrics.SessionGlobalReadLocks.Set(float64(globalReadLockWait))
	s.Metrics.SessionsCopyingToTable.Set(float64(copyToTable))
	s.Metrics.SessionsStatistics.Set(float64(statistics))

	return
}

// GetInnodbStats collects metrics related to InnoDB engine
func (s *MysqlStatDBs) GetInnodbStats() {
	res, err := s.Db.QueryReturnColumnDict(innodbQuery)
	if err != nil {
		s.Db.Log(err)
		return
	}
	var innodbLogFileSize int64
	if err == nil && len(res["Value"]) > 0 {
		innodbLogFileSize, err = strconv.ParseInt(res["Value"][0], 10, 64)
		if err != nil {
			s.Db.Log(err)
		}
	}

	res, err = s.Db.QueryReturnColumnDict("SHOW ENGINE INNODB STATUS")
	if err != nil {
		s.Db.Log(err)
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
				s.Db.Log(err)
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
	if lsn, ok := idb.Metrics["log_sequence_number"]; ok && innodbLogFileSize != 0 {
		lsns, _ := strconv.ParseFloat(lsn, 64)
		s.Metrics.InnodbLogWriteRatio.Set((lsns * 3600.0) / float64(innodbLogFileSize))
	}
	return
}

// GetBackups collects information about backup processes
// TODO: Find a better method than parsing output from ps
func (s *MysqlStatDBs) GetBackups() {
	out, err := exec.Command("ps", "aux").Output()
	if err != nil {
		s.Db.Log(err)
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
			backupProcs++
		}
	}
	s.Metrics.BackupsRunning.Set(float64(backupProcs))
	return
}

// GetSecurity collects information about users without authentication
func (s *MysqlStatDBs) GetSecurity() {
	res, err := s.Db.QueryReturnColumnDict(securityQuery)
	if err != nil {
		s.Db.Log(err)
		return
	}
	unsecureUsers := 0
	if len(res["COUNT(*)"]) > 0 {
		count, err := strconv.ParseInt(res["COUNT(*)"][0], 10, 0)
		if err != nil {
			s.Db.Log(err)
			return
		}
		unsecureUsers = int(count)
	}
	s.Metrics.UnsecureUsers.Set(float64(unsecureUsers))
	return
}

// GetSSL checks whether or not SSL is enabled
func (s *MysqlStatDBs) GetSSL() {
	res, err := s.Db.QueryReturnColumnDict(sslQuery)
	if err != nil {
		s.Db.Log(err)
		return
	}

	if row, ok := res["@@have_ssl"]; !ok || len(row) == 0 {
		s.Db.Log("Mysql does not have the @@have_ssl field!")
		s.Metrics.HasSSL.Set(0)
	} else if row[0] == "YES" {
		s.Metrics.HasSSL.Set(1)
	} else {
		s.Metrics.HasSSL.Set(0)
	}
	return
}

func (s *MysqlStatDBs) GetReadOnly() {

	// Get ReadOnly
	res, err := s.Db.QueryReturnColumnDict(readOnlyQuery)
	if err != nil {
		s.Db.Log(err)
		return
	}
	if row, ok := res["@@read_only"]; !ok || len(row) == 0 {
		s.Db.Log("Mysql does not have the @@read_only field")
		s.Metrics.IsReadOnly.Set(0)
	} else if row[0] == "1" {
		s.Metrics.IsReadOnly.Set(1)
	} else {
		s.Metrics.IsReadOnly.Set(0)
	}

	// Get SuperReadOnly
	res, err = s.Db.QueryReturnColumnDict(superReadOnlyQuery)
	if err != nil {
		s.Db.Log(err)
		return
	}
	if row, ok := res["@@super_read_only"]; !ok || len(row) == 0 {
		s.Db.Log("Mysql does not have the @@super_read_only field")
		s.Metrics.IsSuperReadOnly.Set(0)
	} else if row[0] == "1" {
		s.Metrics.IsSuperReadOnly.Set(1)
	} else {
		s.Metrics.IsSuperReadOnly.Set(0)
	}
	return
}

// FormatGraphite returns []string of metric values of the form:
// "metric_name metric_value"
// This is the form that stats-collector uses to send messages to graphite
func (s *MysqlStatDBs) FormatGraphite(w io.Writer) error {
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
