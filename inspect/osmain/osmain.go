package osmain

import (
	"github.com/square/inspect/os/cpustat"
	"github.com/square/inspect/os/memstat"
	"github.com/square/inspect/os/pidstat"
)

// OsIndependentStats must be implemented by all supported platforms
type OsIndependentStats struct {
	Cstat *cpustat.CPUStat
	Mstat *memstat.MemStat
	Procs *pidstat.ProcessStat
}
