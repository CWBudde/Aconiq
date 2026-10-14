//go:build linux

package runestimate

import (
	"time"

	"golang.org/x/sys/unix"
)

// ThreadCPUTime is the CPU time the calling OS thread has used, and whether
// the platform can say. A goroutine that wants two readings to describe its
// own work must hold its thread between them (runtime.LockOSThread).
func ThreadCPUTime() (time.Duration, bool) {
	var ts unix.Timespec

	err := unix.ClockGettime(unix.CLOCK_THREAD_CPUTIME_ID, &ts)
	if err != nil {
		return 0, false
	}

	return time.Duration(ts.Nano()), true
}
