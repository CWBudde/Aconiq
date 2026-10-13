package road_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	road "github.com/aconiq/backend/internal/standards/rls19/road"
)

// progressReceiverCount is smaller than parallelReceiverCount because these
// tests re-walk the same expensive scene at every worker count under -race,
// and the claim needs ragged chunks, not many of them. Prime, so no worker
// count under test divides it, and large enough that 17 workers still get
// several chunks.
const progressReceiverCount = 101

// Asking for progress must not move a level. The counter is the one piece of
// state the workers share that the walk did not share before, so this is the
// bit-identity claim of parallel_test.go restated with it switched on — at
// every worker count, including the delegating one.
func TestProgressLeavesTheParallelOutputsBitIdentical(t *testing.T) {
	t.Parallel()

	sources, barriers, cfg := parallelScene()
	receivers := parallelReceivers(progressReceiverCount)

	want, err := road.ComputeReceiverOutputs(receivers, sources, barriers, cfg)
	if err != nil {
		t.Fatalf("sequential walk failed: %v", err)
	}

	for _, workers := range parallelWorkerCounts {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			t.Parallel()

			got, err := road.ComputeReceiverOutputsParallelProgress(
				t.Context(), receivers, sources, barriers, cfg, workers,
				func(int, int) {},
			)
			if err != nil {
				t.Fatalf("parallel walk failed: %v", err)
			}

			if !reflect.DeepEqual(got, want) {
				t.Fatalf("workers=%d with progress moved a result", workers)
			}
		})
	}
}

// The sequence ProgressFunc promises: one call per receiver, done counting up
// by one from 1 to total, and total never changing. A pool that let two
// workers report out of order — 8 before 7 — or that reported a receiver
// twice, fails here, under -race if the serialisation is missing altogether.
func TestProgressCountsEveryReceiverOnceAndInOrder(t *testing.T) {
	t.Parallel()

	sources, barriers, cfg := parallelScene()
	receivers := parallelReceivers(progressReceiverCount)

	for _, workers := range parallelWorkerCounts {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			t.Parallel()

			var reports [][2]int

			_, err := road.ComputeReceiverOutputsParallelProgress(
				t.Context(), receivers, sources, barriers, cfg, workers,
				func(done, total int) {
					reports = append(reports, [2]int{done, total})
				},
			)
			if err != nil {
				t.Fatalf("parallel walk failed: %v", err)
			}

			if len(reports) != len(receivers) {
				t.Fatalf("got %d reports, want one per receiver (%d)", len(reports), len(receivers))
			}

			for i, report := range reports {
				if report != [2]int{i + 1, len(receivers)} {
					t.Fatalf("report %d = %v, want [%d %d]", i, report, i+1, len(receivers))
				}
			}
		})
	}
}

// A walk that never starts reports nothing: the cancelled context is refused
// before the first receiver, and a "0 of n" from here would be the walk
// claiming a report it has no receiver for.
func TestProgressIsSilentForACancelledWalk(t *testing.T) {
	t.Parallel()

	sources, barriers, cfg := parallelScene()
	receivers := parallelReceivers(progressReceiverCount)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	for _, workers := range []int{1, 4} {
		calls := 0

		_, err := road.ComputeReceiverOutputsParallelProgress(
			ctx, receivers, sources, barriers, cfg, workers,
			func(int, int) { calls++ },
		)
		if err == nil {
			t.Fatalf("workers=%d: expected the cancelled walk to fail", workers)
		}

		if calls != 0 {
			t.Fatalf("workers=%d: a cancelled walk reported %d times", workers, calls)
		}
	}
}
