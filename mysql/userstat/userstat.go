// Copyright (c) 2015 Square, Inc
//

package userstat

import (
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/mysql/tools"
	"github.com/square/inspect/mysql/util"
	"github.com/square/inspect/os/misc"
)

const (
	innodbMetadataCheck = "SELECT @@GLOBAL.innodb_stats_on_metadata;"
	usrStatisticsQuery  = `
SELECT user, total_connections, concurrent_connections, connected_time, cpu_time
  FROM INFORMATION_SCHEMA.USER_STATISTICS;`
	defaultMaxConns = 5
)

// MysqlStatUsers - main struct that contains connection to database, metric context, and map to stats struct
type MysqlStatUsers struct {
	util.MysqlStat
	Users map[string]*MysqlStatPerUser
	nLock *sync.Mutex
}

// MysqlStatPerUser represents metrics
type MysqlStatPerUser struct {
	TotalConnections *metrics.Counter
	ConnectedTime    *metrics.Counter
	CPUTime          *metrics.Counter
}

// New initializes mysqlstat and returns it
// arguments: metrics context, username, password, path to config file for
// mysql. username and password can be left as "" if a config file is specified.
func New(m *metrics.MetricContext, user, password, host, config string) (*MysqlStatUsers, error) {
	s := new(MysqlStatUsers)
	s.M = m
	s.nLock = &sync.Mutex{}
	// connect to database
	var err error
	s.Db, err = tools.New(user, password, host, config)
	if err != nil { //error in connecting to database
		return nil, err
	}
	s.nLock.Lock()
	s.Users = make(map[string]*MysqlStatPerUser)
	s.nLock.Unlock()
	return s, nil
}

//initialize user statistics
func newMysqlStatPerUser(m *metrics.MetricContext, user string) *MysqlStatPerUser {
	o := new(MysqlStatPerUser)
	misc.InitializeMetrics(o, m, "mysqlstat."+user, true)
	return o
}

// Collect collects various database/table metrics
// sql.DB is thread safe so launching metrics collectors
// in their own goroutines is safe
func (s *MysqlStatUsers) Collect() {
	s.Wg.Add(1)
	go s.GetUserStatistics()
	s.Wg.Wait()
}

//check if database struct is instantiated, and instantiate if not
func (s *MysqlStatUsers) checkUser(user string) {
	s.nLock.Lock()
	if _, ok := s.Users[user]; !ok {
		s.Users[user] = newMysqlStatPerUser(s.M, user)
	}
	s.nLock.Unlock()
	return
}

// GetUserStatistics collects user statistics: user, total connections, concurrent connections, connected time
func (s *MysqlStatUsers) GetUserStatistics() {
	fields := []string{"total_connections", "connected_time", "cpu_time"}

	res, err := s.Db.QueryReturnColumnDict(usrStatisticsQuery)
	if len(res) == 0 || err != nil {
		s.Db.Log(err)
		s.Wg.Done()
		return
	}
	for i, user := range res["user"] {
		s.checkUser(user)
		for _, queryField := range fields {
			field, err := strconv.ParseInt(res[queryField][i], 10, 64)
			if err != nil {
				s.Db.Log(err)
			}
			s.nLock.Lock()
			// cannot use reflection to get a field dynamically because it's too slow
			switch {
			case queryField == "total_connections":
				s.Users[user].TotalConnections.Set(uint64(field))
			case queryField == "connected_time":
				s.Users[user].ConnectedTime.Set(uint64(field))
			case queryField == "cpu_time":
				s.Users[user].CPUTime.Set(uint64(field))
			}
			s.nLock.Unlock()
		}
	}
	s.Wg.Done()
	return
}

// FormatGraphite writes metrics in the form "metric_name metric_value"
// to the input writer
func (s *MysqlStatUsers) FormatGraphite(w io.Writer) error {
	for username, user := range s.Users {
		fmt.Fprintln(w, username+".TotalConnections "+
			strconv.FormatUint(user.TotalConnections.Get(), 10))
		fmt.Fprintln(w, username+".ConnectedTime "+
			strconv.FormatUint(user.ConnectedTime.Get(), 10))
		fmt.Fprintln(w, username+".CPUTime "+
			strconv.FormatUint(user.CPUTime.Get(), 10))
	}
	return nil
}
