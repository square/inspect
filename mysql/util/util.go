package util

import (
	"sync"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/mysql/tools"
)

// MysqlStat represents a connection to the database
type MysqlStat struct {
	M  *metrics.MetricContext
	Db tools.MysqlDB //mysql connection
}

// SetMaxConnections sets the max number of concurrent connections that the mysql client can use
func (s *MysqlStat) SetMaxConnections(maxConns int) {
	s.Db.SetMaxConnections(maxConns)
}

// CollectInParallel takes a list of functions and runs them in parallel, making a waitgroup block until they're done
func CollectInParallel(queryFuncList []func()) {
	var wg sync.WaitGroup
	wg.Add(len(queryFuncList))
	for _, queryFunc := range queryFuncList {
		go func(f func()) {
			f()
			wg.Done()
		}(queryFunc)
	}
	wg.Wait()
}

// Close closes database connection
func (s *MysqlStat) Close() {
	s.Db.Close()
}
