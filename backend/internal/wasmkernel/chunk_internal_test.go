package wasmkernel

import (
	"testing"
	"time"
)

// How the walk resizes its chunks, tested on numbers rather than on a clock.
//
// ComputeRLS19Road's own tests cannot reach this: they measure a real
// computation, so what they could assert about the sizes it settles on is a
// property of the machine running them. The extrapolation itself is
// arithmetic, and pinning it here is what lets those tests confine themselves
// to what holds everywhere — that the first report comes early and the last
// one lands on the total.
func TestNextChunkSize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		current  int
		computed int
		elapsed  time.Duration
		want     int
	}{
		{
			// The interval divided evenly: 8 receivers in 50 ms is 16 in 100.
			name:    "extrapolates to the target interval",
			current: 8, computed: 8, elapsed: 50 * time.Millisecond,
			want: 16,
		},
		{
			// The reported case. A chunk that took twenty seconds must not
			// take another twenty to admit it, so the shrink is immediate and
			// unlimited — down to the floor in one step.
			name:    "collapses to one receiver when a chunk runs far over budget",
			current: 256, computed: 256, elapsed: 20 * time.Second,
			want: 1,
		},
		{
			// Even one receiver over budget: there is nothing smaller to
			// pause at, and reporting per receiver is the most a scene this
			// expensive can offer.
			name:    "floors at one receiver",
			current: 1, computed: 1, elapsed: 5 * time.Second,
			want: 1,
		},
		{
			// A cheap scene asks for tens of thousands; the growth limit lets
			// it have four times what it just did, and the next chunk asks
			// again.
			name:    "rises by no more than the growth limit",
			current: 8, computed: 8, elapsed: time.Microsecond,
			want: 8 * chunkGrowthLimit,
		},
		{
			name:    "never exceeds the cap",
			current: MaxChunkSize, computed: MaxChunkSize, elapsed: time.Microsecond,
			want: MaxChunkSize,
		},
		{
			// Below the clock's resolution is not a measurement of "instant".
			// Growing by the limit tries again with a chunk large enough to
			// time; extrapolating from it would divide by zero.
			name:    "grows by the limit when the chunk was too short to time",
			current: 8, computed: 8, elapsed: 0,
			want: 8 * chunkGrowthLimit,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := nextChunkSize(tc.current, tc.computed, tc.elapsed)
			if got != tc.want {
				t.Fatalf(
					"nextChunkSize(%d, %d, %v) = %d, want %d",
					tc.current, tc.computed, tc.elapsed, got, tc.want,
				)
			}
		})
	}
}
