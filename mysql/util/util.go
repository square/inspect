package util

import (
	"sync"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/mysql/tools"
)

type MysqlStat struct {
	M  *metrics.MetricContext
	Db tools.MysqlDB //mysql connection
	Wg sync.WaitGroup
}

// SetMaxConnections sets the max number of concurrent connections that the mysql client can use
func (s *MysqlStat) SetMaxConnections(maxConns int) {
	s.Db.SetMaxConnections(maxConns)
}

// Close closes database connection
func (s *MysqlStat) Close() {
	s.Db.Close()
}
