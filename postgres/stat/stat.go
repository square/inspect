package stat

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/misc"
	"github.com/square/inspect/postgres/tools"
)

//PostgresStat stores info on the db
type PostgresStat struct {
	Metrics  *PostgresStatMetrics
	m        *metrics.MetricContext
	db       tools.PostgresDB
	DBs      map[string]*DBMetrics
	Modes    map[string]*ModeMetrics
	idleCol  string
	idleStr  string
	queryCol string
	pidCol   string
	PGDATA   string
	dsn      map[string]string
	dbLock   *sync.Mutex
	modeLock *sync.Mutex
	wg       sync.WaitGroup
}

//ModeMetrics - metrics on lock modes
type ModeMetrics struct {
	Locks *metrics.Gauge
}

//DBMetrics - includes metrics for dbs
// and a mapping to tables in the db
type DBMetrics struct {
	Tables    map[string]*TableMetrics
	SizeBytes *metrics.Gauge
}

//TableMetrics - metrics for each table
type TableMetrics struct {
	SizeBytes *metrics.Gauge
}

// PostgresStatMetrics represents metrics collected for postgres database
type PostgresStatMetrics struct {
	Uptime               *metrics.Counter
	Version              *metrics.Gauge
	TPS                  *metrics.Counter
	BlockReadsDisk       *metrics.Counter
	BlockReadsCache      *metrics.Counter
	CacheHitPct          *metrics.Gauge
	CommitRatio          *metrics.Gauge
	WalKeepSegments      *metrics.Gauge
	SessionMax           *metrics.Gauge
	SessionCurrentTotal  *metrics.Gauge
	SessionBusyPct       *metrics.Gauge
	ConnMaxPct           *metrics.Gauge
	OldestTrxS           *metrics.Gauge
	OldestQueryS         *metrics.Gauge
	ActiveLongRunQueries *metrics.Gauge
	LockWaiters          *metrics.Gauge
	CpuPct               *metrics.Gauge
	MemPct               *metrics.Gauge
	VSZ                  *metrics.Gauge
	RSS                  *metrics.Gauge
	UnsecureUsers        *metrics.Gauge
	Writable             *metrics.Gauge //0 if not writable, 1 if writable
	BackupsRunning       *metrics.Gauge
	BinlogFiles          *metrics.Gauge
	DBSizeBinlogs        *metrics.Gauge
	SecondsBehindMaster  *metrics.Gauge
	SlavesConnectedToMe  *metrics.Gauge
	VacuumsAutoRunning   *metrics.Gauge
	VacuumsManualRunning *metrics.Gauge
	SlaveBytesBehindMe   *metrics.Gauge
}

//store all sql commands here
const (
	uptimeQuery = `
  SELECT EXTRACT(epoch FROM now())
       - EXTRACT(epoch From pg_postmaster_start_time()) AS uptime;`
	versionQuery     = "SELECT VERSION() AS version;"
	tpsQuery         = "SELECT SUM(xact_commit + xact_rollback) AS tps FROM pg_stat_database;"
	cacheInfoQuery   = "SELECT SUM(blks_read) AS block_reads_disk, SUM(blks_hit) AS block_reads_cache FROM pg_stat_database;"
	commitRatioQuery = `
        SELECT AVG(ROUND((100.0*sd.xact_commit)/(sd.xact_commit+sd.xact_rollback), 2)) AS commit_ratio
          FROM pg_stat_database sd
          JOIN pg_database d ON (d.oid=sd.datid)
          JOIN pg_user u ON (u.usesysid=d.datdba)
         WHERE sd.xact_commit+sd.xact_rollback != 0;`
	walKeepSegmentsQuery = "SELECT setting FROM pg_settings WHERE name = 'wal_keep_segments';"
	sessionMaxQuery      = "SELECT setting FROM pg_settings WHERE name = 'max_connections';"
	sessionQuery         = `
        SELECT (SELECT COUNT(*) FROM pg_stat_activity 
                 WHERE %s = '%s') AS idle,
               (SELECT COUNT(*) FROM pg_stat_activity 
                 WHERE %s != '%s') AS active;`
	oldestQuery = `
        SELECT EXTRACT(epoch FROM NOW()) - EXTRACT(epoch FROM %s) AS oldest
          FROM pg_stat_activity 
         WHERE %s != '%s'
           AND UPPER(%s) NOT LIKE '%%VACUUM%%'
         ORDER BY 1 DESC LIMIT 1;`
	longEntriesQuery = `
        SELECT * FROM pg_stat_activity 
         WHERE EXTRACT(epoch FROM NOW()) - EXTRACT(epoch FROM query_start) > %s
           AND %s != '%s';`
	lockWaitersQuery = `
        SELECT bl.pid                 AS blocked_pid,
               a.usename              AS blocked_user,
               ka.%s       AS blocking_statement,
               NOW() - ka.query_start AS blocking_duration,
               kl.pid                 AS blocking_pid,
               ka.usename             AS blocking_user,
               a.%s        AS blocked_statement,
               NOW() - a.query_start  AS blocked_duration
          FROM pg_catalog.pg_locks bl
          JOIN pg_catalog.pg_stat_activity a
            ON a.%s = bl.pid
          JOIN pg_catalog.pg_locks kl 
            ON kl.transactionid = bl.transactionid AND kl.pid != bl.pid
          JOIN pg_catalog.pg_stat_activity ka ON ka.%s = kl.pid
         WHERE NOT bl.granted;`
	locksQuery   = "SELECT mode, COUNT(*) AS count FROM pg_locks WHERE granted GROUP BY 1;"
	vacuumsQuery = `
        SELECT %s FROM pg_stat_activity
         WHERE UPPER(%s) LIKE '%%VACUUM%%';`
	dbSizeQuery = `
	SELECT datname AS dbname, PG_DATABASE_SIZE(datname) AS size
	  FROM pg_database;`
	tblSizeQuery = `
          SELECT nspname || '.' || relname AS relation,
                 PG_TOTAL_RELATION_SIZE(C.oid) AS total_size
            FROM pg_class C
            LEFT JOIN pg_namespace N ON (N.oid = C.relnamespace)
           WHERE nspname NOT IN ('pg_catalog', 'information_schema')
             AND C.relkind <> 'i'
             AND nspname !~ '^pg_toast'
           ORDER BY pg_total_relation_size(C.oid) DESC;`
	secondsBehindMasterQuery = `
        SELECT EXTRACT(epoch FROM NOW()) 
             - EXTRACT(epoch FROM pg_last_xact_replay_timestamp()) AS seconds;`
	delayBytesQuery = `
        SELECT pg_current_xlog_location(), write_location, client_hostname
          FROM pg_stat_replication;`
	securityQuery = "SELECT usename FROM pg_shadow WHERE passwd IS NULL;"
)

// New opens connection to postgres database and starts collector for metrics
func New(m *metrics.MetricContext, user, config string) (*PostgresStat, error) {
	s := new(PostgresStat)

	s.dsn = map[string]string{
		"dbname": "postgres",
		"user":   user, "sslmode": "disable",
		"host": "localhost",
	}
	var err error
	s.Modes = make(map[string]*ModeMetrics)
	s.DBs = make(map[string]*DBMetrics)
	s.db, err = tools.New(s.dsn)
	s.dbLock = &sync.Mutex{}
	s.modeLock = &sync.Mutex{}
	s.m = m
	s.PGDATA = "/data/pgsql"
	if err != nil {
		s.db.Log(err)
		return nil, err
	}

	s.Metrics = PostgresStatMetricsNew(m)

	return s, nil
}

// Close closes database connection
func (s *PostgresStat) Close() {
	s.db.Close()
}

// PostgresStatMetricsNew initializes PostgresStatMetrics
func PostgresStatMetricsNew(m *metrics.MetricContext) *PostgresStatMetrics {
	c := new(PostgresStatMetrics)
	misc.InitializeMetrics(c, m, "postgresstat", true)
	return c
}

//checks for initialization of db metrics
func (s *PostgresStat) checkDB(dbname string) {
	s.dbLock.Lock()
	if _, ok := s.DBs[dbname]; !ok {
		o := new(DBMetrics)
		o.Tables = make(map[string]*TableMetrics)
		misc.InitializeMetrics(o, s.m, "postgresstat."+dbname, true)
		s.DBs[dbname] = o
	}
	s.dbLock.Unlock()
}

//checks for initialization of lock mode metrics
func (s *PostgresStat) checkMode(name string) {
	s.modeLock.Lock()
	if _, ok := s.Modes[name]; !ok {
		o := new(ModeMetrics)
		misc.InitializeMetrics(o, s.m, "postgresstat.lock"+name, true)
		s.Modes[name] = o
	}
	s.modeLock.Unlock()
}

//checks for intialization of table metrics
func (s *PostgresStat) checkTable(dbname, tblname string) {
	s.checkDB(dbname)
	s.dbLock.Lock()
	if _, ok := s.DBs[dbname].Tables[tblname]; !ok {
		o := new(TableMetrics)
		misc.InitializeMetrics(o, s.m, "postgresstat."+dbname+"."+tblname, true)
		s.DBs[dbname].Tables[tblname] = o
	}
	s.dbLock.Unlock()
}

// Collect runs metrics collections
func (s *PostgresStat) Collect() {
	s.wg.Add(1)
	s.getVersion()
	s.wg.Wait()
	s.wg.Add(17)
	go s.getUptime()
	go s.getTPS()
	go s.getCacheInfo()
	go s.getCommitRatio()
	go s.getWalKeepSegments()
	go s.getSessions()
	go s.getOldest()
	go s.getNumLongEntries()
	go s.getLocks()
	go s.getVacuumsInProgress()
	go s.getMainProcessInfo()
	go s.getSizes()
	go s.getSecondsBehindMaster()
	go s.getSlaveDelayBytes()
	go s.getSecurity()
	go s.getBackups()
	go s.getWriteability()
	s.wg.Wait()
}

//get uptime
func (s *PostgresStat) getUptime() {
	res, err := s.db.QueryReturnColumnDict(uptimeQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	v, ok := res["uptime"]
	if !ok || len(v) == 0 {
		s.db.Log(errors.New("Couldn't get uptime"))
		s.wg.Done()
		return
	}
	time, err := strconv.ParseFloat(v[0], 64)
	s.Metrics.Uptime.Set(uint64(time))
	if err != nil {
		s.db.Log(err)
	}
	s.wg.Done()
	return
}

//get version
//looks like:
//  'PostgreSQL 9.1.5 on x86_64-unknown-linux-gnu....'
func (s *PostgresStat) getVersion() {
	res, err := s.db.QueryReturnColumnDict(versionQuery)
	if err != nil || len(res["version"]) == 0 {
		s.db.Log(errors.New("Couldn't get version"))
		s.wg.Done()
		return
	}
	version := res["version"][0]
	version = strings.Split(version, " ")[1]
	leading := float64(len(strings.Split(version, ".")[0]))
	version = strings.Replace(version, ".", "", -1)
	ver, err := strconv.ParseFloat(version, 64)
	ver /= math.Pow(10.0, float64(len(version))-leading)
	s.Metrics.Version.Set(ver)
	if ver >= 9.2 {
		s.pidCol = "pid"
		s.queryCol = "query"
		s.idleCol = "state"
		s.idleStr = "idle"
	} else {
		s.pidCol = "procpid"
		s.queryCol = "current_query"
		s.idleCol = s.queryCol
		s.idleStr = "<IDLE>"
	}
	if err != nil {
		s.db.Log(err)
	}
	s.wg.Done()
	return
}

//get TPS
func (s *PostgresStat) getTPS() {
	res, err := s.db.QueryReturnColumnDict(tpsQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	v, ok := res["tps"]
	if !ok || len(v) == 0 {
		s.db.Log(errors.New("Unable to get tps"))
		s.wg.Done()
		return
	}
	val, err := strconv.ParseInt(v[0], 10, 64)
	s.Metrics.TPS.Set(uint64(val))
	if err != nil {
		s.db.Log(err)
	}
	s.wg.Done()
	return

}

//get cache info
func (s *PostgresStat) getCacheInfo() {
	res, err := s.db.QueryReturnColumnDict(cacheInfoQuery)
	disk := int64(0)
	cache := int64(0)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	if len(res["block_reads_disk"]) == 0 {
		s.db.Log("cannot get block_reads_disk")
	} else {
		disk, err = strconv.ParseInt(res["block_reads_disk"][0], 10, 64)
		if err != nil {
			s.db.Log(err)
		}
		s.Metrics.BlockReadsDisk.Set(uint64(disk))
	}
	if len(res["block_reads_cache"]) == 0 {
		s.db.Log("cannot get block_reads_cache")
	} else {
		cache, err = strconv.ParseInt(res["block_reads_cache"][0], 10, 64)
		if err != nil {
			s.db.Log(err)
		}
		s.Metrics.BlockReadsCache.Set(uint64(cache))
	}
	if int64(disk) != int64(0) && int64(cache) != int64(0) {
		pct := (float64(cache) / float64(cache+disk)) * 100.0
		s.Metrics.CacheHitPct.Set(float64(pct))
	}
	if err != nil {
		s.db.Log(err)
	}
	s.wg.Done()
	return
}

//get commit ratio
func (s *PostgresStat) getCommitRatio() {
	res, err := s.db.QueryReturnColumnDict(commitRatioQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	v, ok := res["commit_ratio"]
	if !ok || len(v) == 0 {
		s.db.Log(errors.New("Can't get commit ratio"))
		s.wg.Done()
		return
	}
	val, err := strconv.ParseFloat(v[0], 64)
	s.Metrics.CommitRatio.Set(val)
	if err != nil {
		s.db.Log(err)
	}
	s.wg.Done()
	return
}

//get wal keep segments
func (s *PostgresStat) getWalKeepSegments() {
	res, err := s.db.QueryReturnColumnDict(walKeepSegmentsQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	v, ok := res["setting"]
	if !ok || len(v) == 0 {
		s.db.Log(errors.New("Can't get WalKeepSegments"))
		s.wg.Done()
		return
	}
	val, err := strconv.ParseFloat(v[0], 64)
	s.Metrics.WalKeepSegments.Set(float64(val))
	if err != nil {
		s.db.Log(err)
	}
	s.wg.Done()
	return
}

//get session info
func (s *PostgresStat) getSessions() {
	res, err := s.db.QueryReturnColumnDict(sessionMaxQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	v, ok := res["setting"]
	if !ok || len(v) == 0 {
		s.db.Log(errors.New("Can't get session max"))
		s.wg.Done()
		return
	}
	sessMax, err := strconv.ParseInt(v[0], 10, 64)
	if err != nil {
		s.db.Log(err)
	}
	s.Metrics.SessionMax.Set(float64(sessMax))

	cmd := fmt.Sprintf(sessionQuery, s.idleCol, s.idleStr, s.idleCol, s.idleStr)

	res, err = s.db.QueryReturnColumnDict(cmd)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	idle := int64(0)
	v, ok = res["idle"]
	if ok || len(v) > 0 {
		idle, err = strconv.ParseInt(v[0], 10, 64)
		if err != nil {
			s.db.Log(err)
		}
	}
	active := int64(0)
	v, ok = res["active"]
	if ok || len(v) > 0 {
		active, err = strconv.ParseInt(v[0], 10, 64)
		if err != nil {
			s.db.Log(err)
		}
	}
	total := float64(active + idle)
	s.Metrics.SessionCurrentTotal.Set(total)
	s.Metrics.SessionBusyPct.Set((float64(active) / total) * 100)
	s.Metrics.ConnMaxPct.Set(float64(total/float64(sessMax)) * 100.0)
	s.wg.Done()
	return
}

//get oldest transaction info
func (s *PostgresStat) getOldest() {
	info := map[string]*metrics.Gauge{"xact_start": s.Metrics.OldestTrxS, "query_start": s.Metrics.OldestQueryS}
	for col, metric := range info {
		cmd := fmt.Sprintf(oldestQuery, col, s.idleCol, s.idleStr, s.queryCol)
		res, err := s.db.QueryReturnColumnDict(cmd)
		if err != nil {
			s.db.Log(err)
			s.wg.Done()
			return
		}
		v, ok := res["oldest"]
		if !ok || len(v) == 0 {
			continue
		}
		if v[0] == "" {
			metric.Set(float64(0))
			continue
		}
		val, err := strconv.ParseFloat(v[0], 64)
		if err != nil {
			s.db.Log(err)
		}
		metric.Set(val)
	}
	s.wg.Done()
	return
}

//get long active running queries
func (s *PostgresStat) getNumLongEntries() {
	threshold := "30"
	cmd := fmt.Sprintf(longEntriesQuery, threshold, s.idleCol, s.idleStr)
	res, err := s.db.QueryReturnColumnDict(cmd)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	if len(res) == 0 {
		s.db.Log(errors.New("can't get num long entries"))
		s.wg.Done()
		return
	}
	for _, col := range res {
		s.Metrics.ActiveLongRunQueries.Set(float64(len(col)))
		s.wg.Done()
		return
	}
	s.wg.Done()
	return
}

//get count on each type of lock held
func (s *PostgresStat) getLocks() {
	cmd := fmt.Sprintf(lockWaitersQuery, s.queryCol, s.queryCol, s.pidCol, s.pidCol)
	res, err := s.db.QueryReturnColumnDict(cmd)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	for _, col := range res {
		s.Metrics.LockWaiters.Set(float64(len(col)))
		break
	}

	res, err = s.db.QueryMapFirstColumnToRow(locksQuery)
	for mode, locks := range res {
		if len(locks) == 0 {
			continue
		}
		lock, _ := strconv.ParseInt(locks[0], 10, 64)
		s.checkMode(mode)
		s.modeLock.Lock()
		s.Modes[mode].Locks.Set(float64(lock))
		s.modeLock.Unlock()
	}
	s.wg.Done()
	return
}

//get vacuum info
func (s *PostgresStat) getVacuumsInProgress() {
	cmd := fmt.Sprintf(vacuumsQuery, s.queryCol, s.queryCol)
	res, err := s.db.QueryReturnColumnDict(cmd)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	auto := 0
	manual := 0
	for _, querC := range res[s.queryCol] {
		if strings.Contains(querC, "datfrozenxid") {
			continue
		}
		m := regexp.MustCompile("(?i)(\\s*autovacuum:\\s*)?(\\s*VACUUM\\s*)?(\\s*ANALYZE\\s*)?\\s*(.+?)$").FindStringSubmatch(querC)

		//TODO: extras
		if len(m) > 0 {
			if strings.HasPrefix(querC, "autovacuum:") {
				auto++
			} else {
				manual++
			}
		}
	}
	s.Metrics.VacuumsAutoRunning.Set(float64(auto))
	s.Metrics.VacuumsManualRunning.Set(float64(manual))
	s.wg.Done()
	return
}

//get process info
func (s *PostgresStat) getMainProcessInfo() {
	out, err := exec.Command("ps", "aux").Output()
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	blob := string(out)
	lines := strings.Split(blob, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		words := strings.Fields(line)

		if len(words) < 10 {
			continue
		}
		cmd := strings.Join(words[10:], " ")

		if strings.Contains(cmd, "postmaster") {
			info := make([]string, 10)
			//mapping for info: 0-user, 1-pid, 2-cpu, 3-mem, 4-vsz, 5-rss, 6-tty, 7-stat, 8-start, 9-time, 10-cmd
			for i, word := range words {
				if i == 10 {
					break
				}
				info[i] = word
			}

			cpu, _ := strconv.ParseFloat(info[2], 64) //TODO: correctly handle these errors
			mem, _ := strconv.ParseFloat(info[3], 64)
			vsz, _ := strconv.ParseFloat(info[4], 64)
			rss, _ := strconv.ParseFloat(info[5], 64)
			s.Metrics.CpuPct.Set(cpu)
			s.Metrics.MemPct.Set(mem)
			s.Metrics.VSZ.Set(vsz)
			s.Metrics.RSS.Set(rss)
		}
	}
	s.wg.Done()
	return
}

//get writeability
// by creating a schema, creating a table, adding to that table and then deleting it
func (s *PostgresStat) getWriteability() {
	_, err := s.db.QueryReturnColumnDict("CREATE SCHEMA postgres_health;")
	if err != nil {
		s.Metrics.Writable.Set(float64(0))
	}
	cmd := `
        CREATE TABLE IF NOT EXISTS postgres_health.postgres_health 
         (id INT PRIMARY KEY, stamp TIMESTAMP);`
	_, err = s.db.QueryReturnColumnDict(cmd)
	if err != nil {
		if strings.Contains(err.Error(), "read-only") {
			s.Metrics.Writable.Set(float64(0))
			s.wg.Done()
			return
		} else if strings.Contains(err.Error(), "already exists") {
			s.db.Log("writeability check failed, test schema already exists")
			s.wg.Done()
			return
		}
		s.db.QueryReturnColumnDict("ROLLBACK;")
	}
	cmd = `
        CREATE TABLE IF NOT EXISTS postgres_health.postgres_health 
        (id INT PRIMARY KEY, stamp TIMESTAMP);`
	s.db.QueryReturnColumnDict(cmd)
	cmd = `
        BEGIN;
        DELETE FROM postgres_health.postgres_health;
        INSERT INTO postgres_health.postgres_health VALUES (1, NOW());
        COMMIT;`
	s.db.QueryReturnColumnDict(cmd)
	s.Metrics.Writable.Set(float64(1))
	s.wg.Done()
	return
}

//get size of tables and databases and binlogs
func (s *PostgresStat) getSizes() {
	//get binlog sizes
	out, err := exec.Command("ls", "-l", "/data/pgsql/pg_xlog/").Output()
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	blob := string(out)
	count := 0
	total := float64(0)
	for _, line := range strings.Split(blob, "\n") {
		cols := strings.Split(line, " ")
		if len(cols) < 5 {
			continue
		}
		count++
		tmp, _ := strconv.ParseFloat(cols[4], 64)
		total += tmp
	}
	s.Metrics.BinlogFiles.Set(float64(count))
	s.Metrics.DBSizeBinlogs.Set(float64(total))

	//get database sizes
	//method similar here to the mysql one
	res, err := s.db.QueryMapFirstColumnToRow(dbSizeQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	for key, value := range res {
		//key being the name of the db, value its size in bytes
		dbname := strings.TrimSpace(string(key))
		size, err := strconv.ParseInt(string(value[0]), 10, 64)
		if err != nil {
			s.db.Log(err)
		}
		if size > 0 {
			s.checkDB(dbname)
			s.dbLock.Lock()
			s.DBs[dbname].SizeBytes.Set(float64(size))
			s.dbLock.Unlock()
		}
	}

	//get table sizes
	for dbname := range res {
		newDsn := make(map[string]string)
		for k, v := range s.dsn {
			newDsn[k] = v
		}
		newDsn["dbname"] = dbname
		newDB, err := tools.New(newDsn)
		if err != nil {
			s.db.Log("Cannot connect to database: " + dbname)
			continue
		}
		cmd := fmt.Sprintf(tblSizeQuery, dbname)
		res, err := newDB.QueryMapFirstColumnToRow(cmd)
		if err != nil {
			s.db.Log(err)
		}
		for relation, sizes := range res {
			size, _ := strconv.ParseInt(sizes[0], 10, 64)
			if size > 0 {
				s.checkTable(dbname, relation)
				s.dbLock.Lock()
				s.DBs[dbname].Tables[relation].SizeBytes.Set(float64(size))
				s.dbLock.Unlock()
			}
		}
		newDB.Close()
	}
	s.wg.Done()
	return
}

//get count of backups
func (s *PostgresStat) getBackups() {
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
		if strings.Contains(command, "pg_dump") {
			backupProcs++
		}
	}
	s.Metrics.BackupsRunning.Set(float64(backupProcs))
	s.wg.Done()
	return
}

//get seconds
func (s *PostgresStat) getSecondsBehindMaster() {
	recoveryConfFile := s.PGDATA + "/recovery.conf"
	recoveryDoneFile := s.PGDATA + "/recovery.done"

	res, err := s.db.QueryReturnColumnDict(secondsBehindMasterQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	v, ok := res["seconds"]
	if !ok || len(v) == 0 {
		s.db.Log(errors.New("Unable to get seconds behind master"))
		s.wg.Done()
		return
	}
	if res["seconds"][0] == "" {
		s.Metrics.SecondsBehindMaster.Set(float64(0)) // or -1?
		s.wg.Done()
		return
	}
	seconds, err := strconv.ParseInt(res["seconds"][0], 10, 64)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	s.Metrics.SecondsBehindMaster.Set(float64(seconds))
	_, confErr := os.Stat(recoveryConfFile)
	if confErr == nil {
		s.Metrics.SecondsBehindMaster.Set(float64(-1))
	}
	_, doneErr := os.Stat(recoveryDoneFile)
	if doneErr == nil && os.IsNotExist(confErr) {
		s.Metrics.SecondsBehindMaster.Set(float64(-1))
	}
	s.wg.Done()
	return
}

//get bytes slave is behind master
func (s *PostgresStat) getSlaveDelayBytes() {

	res, err := s.db.QueryReturnColumnDict(delayBytesQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	s.Metrics.SlavesConnectedToMe.Set(float64(len(res["client_hostname"])))
	for _, val := range res["pg_current_xlog_location"] {
		str := strings.Split(val, "/")
		if len(str) < 2 {
			s.db.Log(errors.New("Can't get slave delay bytes"))
			s.wg.Done()
			return
		}
		var masterFile, masterPos, slaveFile, slavePos int64
		masterFile, err = strconv.ParseInt(str[0], 16, 64)

		masterPos, err = strconv.ParseInt(str[1], 16, 64)

		str2 := strings.Split(res["write_location"][0], "/")
		if len(str2) < 2 {
			s.db.Log(errors.New("Can't get slave delay bytes"))
			s.wg.Done()
			return
		}
		slaveFile, err = strconv.ParseInt(str2[0], 16, 64)

		slavePos, err = strconv.ParseInt(str2[1], 16, 64)

		segmentSize, _ := strconv.ParseInt("0xFFFFFFFF", 0, 64)

		r := ((masterFile * segmentSize) + masterPos) - ((slaveFile * segmentSize) + slavePos)
		s.Metrics.SlaveBytesBehindMe.Set(float64(r))
	}
	if err != nil {
		s.db.Log(err)
	}
	s.wg.Done()
	return
}

//get count of users without passwords
func (s *PostgresStat) getSecurity() {
	res, err := s.db.QueryReturnColumnDict(securityQuery)
	if err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	if len(res) > 0 {
		s.Metrics.UnsecureUsers.Set(float64(len(res["usename"])))
	}
	s.wg.Done()
	return
}
