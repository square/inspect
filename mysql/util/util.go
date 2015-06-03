package util

import (
	"errors"
	"reflect"
	"regexp"
	"strings"
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

// CallByMethodName searches for a method implemented
// by s with name. Runs all methods that match names.
func (s *MysqlStat) CallByMethodName(name string) error {
	r := reflect.TypeOf(s)
	re := regexp.MustCompile(strings.ToLower(name))
	f := false
	for i := 0; i < r.NumMethod(); i++ {
		n := strings.ToLower(r.Method(i).Name)
		if strings.Contains(n, "get") && re.MatchString(n) {
			s.Wg.Add(1)
			reflect.ValueOf(s).Method(i).Call([]reflect.Value{})
			f = true
		}
	}
	if !f {
		return errors.New("Could not find function")
	}
	return nil
}
