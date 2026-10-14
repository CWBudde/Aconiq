package road

import "sync"

// ProgressFunc is told how far a walk has got: done receivers computed out of
// total.
//
// The shape is wasmkernel.ComputeRLS19Road's, so the two targets report in one
// vocabulary: never before the first receiver has completed, done strictly
// increasing from one call to the next, and exactly one call with
// done == total on a walk that succeeds. A walk that fails or is cancelled
// stops reporting wherever it stopped computing.
//
// It is never called concurrently, whichever goroutine finished the receiver
// being reported — the pool serialises the calls — so an implementation may
// write a file or a log without a lock of its own. It is called from inside
// the pool, though, and holds a worker up for as long as it takes: an
// implementation that does I/O on every call should throttle.
type ProgressFunc func(done, total int)

// progressCounter turns "one more receiver is done", told from any number of
// goroutines, into the serialised and monotone sequence ProgressFunc promises.
//
// It is the only state the workers share that the walk did not already share,
// and it is write-only from the walk's side: no receiver's computation reads
// it, so it cannot move a level. A nil counter is valid and counts nothing,
// which is how every caller that asked for no progress pays nothing for it.
type progressCounter struct {
	mu       sync.Mutex
	done     int
	total    int
	progress ProgressFunc
}

func newProgressCounter(total int, progress ProgressFunc) *progressCounter {
	if progress == nil {
		return nil
	}

	return &progressCounter{total: total, progress: progress}
}

// add records n more completed receivers and reports the new count.
//
// The call to progress happens under the lock, which is what makes the
// sequence monotone: two workers finishing at once would otherwise be able to
// report 7 after 8.
func (c *progressCounter) add(n int) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.done += n
	c.progress(c.done, c.total)
}
