// Copyright (c) 2015 Square, Inc

// Package loadstat implements metrics collection related to loadavg
package loadstat

import (
	"bufio"
	"github.com/kr/pty"
	"os/exec"
	"strings"
	"time"

	"github.com/square/inspect/metrics"
	"github.com/square/inspect/os/misc"
)

// Collect populates Loadstat by using sysctl
func (s *LoadStat) CollectDarwin() {
	cmd := exec.Command("sysctl", "vm.loadavg")
	tty, err := pty.Start(cmd)
	if err != nil {
		panic(err)
	}
	defer tty.Close()

	scanner := bufio.NewScanner(tty)
	for scanner.Scan() {
		f := strings.Split(scanner.Text(), " ")
		if len(f) > 2 {
			fmt.Println(f)
			s.OneMinute.Set(misc.ParseFloat(f[2]))
			s.FiveMinute.Set(misc.ParseFloat(f[3]))
			s.FifteenMinute.Set(misc.ParseFloat(f[4]))
		}
		break
	}
}
