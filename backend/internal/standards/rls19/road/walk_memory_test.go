package road

import (
	"runtime"
	"testing"
)

// The walk must not hold on to its contributions. On a building-dense model
// in fixed mode a receiver sums one term per Teilstück and per mirrored path —
// ~44 000 Teilstücke times thousands of paths on a city extract — and the
// walk used to collect them all into a slice per period before summing,
// gigabytes per receiver and per worker. These tests pin that the walk's
// allocation per receiver does not grow with the number of terms it sums.

// walkAllocation returns the bytes allocated walking receivers, after one
// warm-up receiver has grown the scratch's buffers, and how many mirrored
// paths the walked receivers summed.
func walkAllocation(t *testing.T, segmentLengthM float64) (uint64, int) {
	t.Helper()

	city := smallCity().build()
	cfg := city.config(segmentLengthM, SegmentLengthFixed)

	scene, err := prepareScene(city.sources, city.barriers, cfg)
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	scratch := scene.newScratch()

	receiverCfg := cfg
	receiverCfg.ReceiverHeightM = 4

	_, err = scene.receiverLevelsOn(city.receivers[0].Point, receiverCfg, scratch)
	if err != nil {
		t.Fatalf("warm-up walk failed: %v", err)
	}

	paths := 0

	for _, receiver := range city.receivers[1:] {
		for _, source := range scene.sources {
			for _, seg := range source.segments {
				paths += len(scene.reflectors.appendPaths(scratch.reflectedPaths[:0], seg.MidPoint, seg.MidZ+pointSourceHeightM, receiver.Point, 4, scratch))
			}
		}
	}

	var before, after runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&before)

	for _, receiver := range city.receivers[1:] {
		_, err = scene.receiverLevelsOn(receiver.Point, receiverCfg, scratch)
		if err != nil {
			t.Fatalf("walk failed: %v", err)
		}
	}

	runtime.ReadMemStats(&after)

	return after.TotalAlloc - before.TotalAlloc, paths
}

func TestReceiverWalkAllocationDoesNotGrowWithContributions(t *testing.T) {
	// Not parallel: TotalAlloc counts every goroutine's allocations.
	fine, finePaths := walkAllocation(t, 2)
	coarse, coarsePaths := walkAllocation(t, 16)

	t.Logf("2 m: %d mirrored paths summed, %d bytes allocated; 16 m: %d paths, %d bytes",
		finePaths, fine, coarsePaths, coarse)

	if finePaths < 4*coarsePaths {
		t.Fatalf("the fine split must sum many more terms than the coarse one: %d vs %d", finePaths, coarsePaths)
	}

	// Holding the terms would cost at least 16 bytes each, for the two
	// periods. The walk may allocate a little — a buffer growing for a
	// receiver that needs more — but nothing that scales with the terms.
	if fine >= uint64(finePaths) {
		t.Fatalf("walking %d mirrored paths allocated %d bytes; the walk is holding its contributions", finePaths, fine)
	}

	if fine > coarse+1<<20 {
		t.Fatalf("the 2 m walk allocated %d bytes against %d at 16 m; allocation grows with the terms summed", fine, coarse)
	}
}
