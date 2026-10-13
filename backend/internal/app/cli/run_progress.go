package cli

import (
	"context"
	"sync"
	"time"

	"github.com/aconiq/backend/internal/io/projectfs"
)

// runProgressInterval is how often a run reports its progress, at most, between
// its first report and its last. Once a second is what a person watching a bar
// reads as moving, and it bounds the cost to one small file and one log line a
// second however cheap a receiver turns out to be.
const runProgressInterval = time.Second

// runProgress is a run's progress feed: a "computed X of Y receivers" line in
// run.log and a progress.json beside it, both kept current while the run
// computes.
//
// It exists because a run that is computing is otherwise silent, and the
// server that started it can only read files. The log line is for a person
// reading the log; progress.json is for a program — the API's run resource —
// and carries the rate-based estimate the line does not.
//
// Reports are throttled to runProgressInterval, except the first and the last,
// which are always written: the first so the size of the job is known as soon
// as it starts, the last so a finished walk never reads as 129 of 130.
//
// A failure to write the file is swallowed. Progress is a report about the
// run, and a run must not fail because its report could not be written.
type runProgress struct {
	store projectfs.Store
	runID string
	log   *runLog
	now   func() time.Time
	// observe is the test hook carried by the command's context; nil in
	// production.
	observe progressObserver

	mu        sync.Mutex
	startedAt time.Time
	lastAt    time.Time
	lastDone  int
	started   bool
}

// progressObserver is told each time a report has been written, with the
// report's numbers. It lets a test look at run.log and progress.json *while*
// the run is computing, which is the property under test, without a sleep.
type progressObserver func(done, total int)

type progressObserverKey struct{}

// withProgressObserver returns a context whose `aconiq run` calls observe after
// every progress report it writes.
func withProgressObserver(ctx context.Context, observe progressObserver) context.Context {
	return context.WithValue(ctx, progressObserverKey{}, observe)
}

func newRunProgress(ctx context.Context, store projectfs.Store, runID string, log *runLog) *runProgress {
	observe, _ := ctx.Value(progressObserverKey{}).(progressObserver)

	return &runProgress{
		store:   store,
		runID:   runID,
		log:     log,
		now:     nowUTC,
		observe: observe,
	}
}

// start records that a walk over total receivers has begun, and writes the
// 0-of-total report. Calling it is what makes a report's estimate measure the
// walk alone, not the model load and scene preparation before it.
func (p *runProgress) start(total int) {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.startedAt = p.now()
	p.started = true
	p.lastDone = -1

	p.write(0, total, p.startedAt)
}

// report is the road.ProgressFunc of a run. It may be called from any
// goroutine; it must not be called concurrently with itself, which the
// compute pools guarantee, but it locks regardless so a module that does not
// cannot corrupt the feed.
func (p *runProgress) report(done, total int) {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		p.startedAt = p.now()
		p.started = true
		p.lastDone = -1
	}

	if done == p.lastDone {
		return
	}

	now := p.now()
	if done < total && p.lastDone >= 0 && now.Sub(p.lastAt) < runProgressInterval {
		return
	}

	p.write(done, total, now)
}

// write puts one report on disk. The caller holds p.mu.
func (p *runProgress) write(done, total int, now time.Time) {
	p.lastAt = now
	p.lastDone = done

	p.log.addAtf(now, "computed %d of %d receivers", done, total)

	_ = p.store.WriteRunProgress(p.runID, projectfs.NewRunProgress(done, total, p.startedAt, now))

	if p.observe != nil {
		p.observe(done, total)
	}
}
