// Copyright (c) 2015 Square, Inc
//

package userstat

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/sorawee/inspect/metrics"
	"github.com/sorawee/inspect/mysql/tools"
	"github.com/sorawee/inspect/os/misc"
)

const (
	innodbMetadataCheck = "SELECT @@GLOBAL.innodb_stats_on_metadata;"
	usrStatisticsQuery  = `
SELECT user, total_connections, concurrent_connections, connected_time
  FROM INFORMATION_SCHEMA.USER_STATISTICS;`
	defaultMaxConns = 5
)

// MysqlStatTables - main struct that contains connection to database, metric context, and map to database stats struct
type MysqlStatTables struct {
	Users map[string]*MysqlUserStats
	m     *metrics.MetricContext
	db    tools.MysqlDB
	nLock *sync.Mutex
	wg    sync.WaitGroup
}

// MysqlUserStats represents metrics
type MysqlUserStats struct {
	TotalConnections      *metrics.Counter
	ConcurrentConnections *metrics.Counter
	ConnectedTime         *metrics.Counter
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
	s.Users = make(map[string]*MysqlUserStats)
	s.nLock.Unlock()
	if err != nil { //error in connecting to database
		return nil, err
	}
	return s, nil
}

// SetMaxConnections sets the max number of concurrent connections that the mysql client can use
func (s *MysqlStatTables) SetMaxConnections(maxConns int) {
	s.db.SetMaxConnections(maxConns)
}

//initialize user statistics
func newMysqlUserStats(m *metrics.MetricContext, user string) *MysqlUserStats {
	o := new(MysqlUserStats)
	misc.InitializeMetrics(o, m, "mysqlstat."+user, true)
	return o
}

// Collect collects various database/table metrics
// sql.DB is thread safe so launching metrics collectors
// in their own goroutines is safe
func (s *MysqlStatTables) Collect() {
	s.wg.Add(1)
	go s.GetUserStatistics()
	s.wg.Wait()
}

//check if database struct is instantiated, and instantiate if not
func (s *MysqlStatTables) checkUser(user string) {
	s.nLock.Lock()
	if _, ok := s.Users[user]; !ok {
		s.Users[user] = newMysqlUserStats(s.m, user)
	}
	s.nLock.Unlock()
	return
}

// GetUserStatistics collects user statistics: user, total connections, concurrent connections, connected time
func (s *MysqlStatTables) GetUserStatistics() {
	fields := []string{"total_connections", "concurrent_connections", "connected_time"}

	res, err := s.db.QueryReturnColumnDict(usrStatisticsQuery)
	if len(res) == 0 || err != nil {
		s.db.Log(err)
		s.wg.Done()
		return
	}
	for i, user := range res["user"] {
		for _, queryField := range fields {
			field, err := strconv.ParseInt(res[queryField][i], 10, 64)
			if err != nil {
				s.db.Log(err)
			}
			s.checkUser(user)
			s.nLock.Lock()
			// cannot use reflection to get a field dynamically because it's too slow
			switch {
			case queryField == "total_connections":
				s.Users[user].TotalConnections.Set(uint64(field))
			case queryField == "concurrent_connections":
				s.Users[user].ConcurrentConnections.Set(uint64(field))
			case queryField == "connected_time":
				s.Users[user].ConnectedTime.Set(uint64(field))
			}
			s.nLock.Unlock()
		}
	}
	s.wg.Done()
	return
}

// Close closes connection with database
func (s *MysqlStatTables) Close() {
	s.db.Close()
}

// CallByMethodName searches for a method implemented
// by s with name. Runs all methods that match names.
func (s *MysqlStatTables) CallByMethodName(name string) error {
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

// FormatGraphite writes metrics in the form "metric_name metric_value"
// to the input writer
func (s *MysqlStatTables) FormatGraphite(w io.Writer) error {
	for username, user := range s.Users {
		fmt.Fprintln(w, username+".TotalConnections "+
			strconv.FormatUint(user.TotalConnections.Get(), 10))
		fmt.Fprintln(w, username+".ConcurrentConnections "+
			strconv.FormatUint(user.ConcurrentConnections.Get(), 10))
		fmt.Fprintln(w, username+".ConnectedTime "+
			strconv.FormatUint(user.ConnectedTime.Get(), 10))
	}
	return nil
}
