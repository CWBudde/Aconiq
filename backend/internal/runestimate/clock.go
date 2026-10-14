package runestimate

import "time"

// The probe times units on the CPU clock of the thread running it, where the
// platform has one, and on the wall clock otherwise.
//
// # Why not the wall clock
//
// A probe lasts a few seconds and a run lasts minutes to hours. Whatever else
// the machine does during those few seconds — another process starting, a
// test suite running beside it, a burst of I/O — lands in a wall-clock
// sample as if it were the cost of the unit, and is then extrapolated over
// the whole run: a probe descheduled for half its window reads every unit as
// twice its cost. The run, which averages over a far longer stretch, sees
// that disturbance only for as long as it lasts. Thread CPU time counts only
// the time the probe actually computed, which is the cost the estimate
// extrapolates; how much of a core the run will get is the business of the
// parallel-efficiency range in the band, not of the sample.
//
// The budget and every deadline stay on the wall clock: they bound how long
// the caller waits, which is wall time by definition.

// unitClock reads the probe's clock. Its zero is arbitrary; only
// differences between two readings on the same goroutine mean anything, and
// on the CPU clock only while that goroutine stays on one OS thread — which
// Run arranges with runtime.LockOSThread.
type unitClock func() time.Duration

// newUnitClock returns the thread CPU clock where there is one, and the
// monotonic wall clock otherwise. cpu reports which.
func newUnitClock() (unitClock, bool) {
	if _, ok := ThreadCPUTime(); ok {
		return func() time.Duration {
			elapsed, _ := ThreadCPUTime()

			return elapsed
		}, true
	}

	origin := time.Now()

	return func() time.Duration { return time.Since(origin) }, false
}
