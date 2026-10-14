//go:build !linux

package runestimate

import "time"

// ThreadCPUTime reports that this platform has no per-thread CPU clock the
// probe reads; it times on the wall clock instead. js/wasm is one such
// platform, and there the browser Worker the probe runs on is its own
// thread, so the wall clock is the nearest substitute there is.
func ThreadCPUTime() (time.Duration, bool) {
	return 0, false
}
