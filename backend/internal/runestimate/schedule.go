package runestimate

import (
	"github.com/aconiq/backend/internal/partition"
)

// Schedule names how a run hands its chunks to its workers.
type Schedule int

const (
	// SchedulePull is the native pool: workers pull the next chunk, in index
	// order, as they finish one (rls19road.ComputeReceiverOutputsParallel).
	SchedulePull Schedule = iota

	// ScheduleSharded is the browser pool: chunk c belongs to shard
	// c mod workers from the start (partition.ForShard), and no Worker takes
	// another's chunk however early it finishes.
	ScheduleSharded
)

// makespan is how long the run's compute takes on workers, given what each
// receiver costs.
//
// It replays the run's own partition rather than dividing by the worker count,
// because the two differ by more than a rounding error exactly where an
// estimate matters: a 130-receiver grid on twelve workers is thirteen chunks
// of ten, so one worker walks two chunks and the run takes twice what
// "CPU / 12" says. partition.Size and partition.Over are the functions the
// pool calls, so the chunks here are the run's chunks.
func makespan(costs []float64, workers int, schedule Schedule) float64 {
	if len(costs) == 0 {
		return 0
	}

	chunks := partition.Over(len(costs), partition.Size(len(costs), workers))

	chunkCost := make([]float64, len(chunks))

	for i, c := range chunks {
		for _, cost := range costs[c.Start:c.End] {
			chunkCost[i] += cost
		}
	}

	if workers <= 1 || len(chunks) <= 1 {
		total := 0.0
		for _, c := range chunkCost {
			total += c
		}

		return total
	}

	finish := make([]float64, min(workers, len(chunks)))

	for i, cost := range chunkCost {
		worker := i % len(finish)

		if schedule == SchedulePull {
			worker = earliest(finish)
		}

		finish[worker] += cost
	}

	longest := 0.0
	for _, f := range finish {
		longest = max(longest, f)
	}

	return longest
}

// earliest is the worker that frees up first, ties to the lower index — the
// one a pull pool hands the next chunk to.
func earliest(finish []float64) int {
	best := 0

	for i, f := range finish {
		if f < finish[best] {
			best = i
		}
	}

	return best
}
