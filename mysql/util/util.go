package util

import (
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/mysql/tools"
)

type MysqlStat {
	m  *metrics.MetricContext
	db tools.MysqlDB //mysql connection
	wg sync.WaitGroup
}

// SetMaxConnections sets the max number of concurrent connections that the mysql client can use
func (s *MysqlStat) SetMaxConnections(maxConns int) {
	s.db.SetMaxConnections(maxConns)
}

// Close closes database connection
func (s *MysqlStat) Close() {
	s.db.Close()
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
