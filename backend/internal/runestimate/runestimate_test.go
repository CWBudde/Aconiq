package runestimate

import (
	"context"
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/geo"
)

func gridPoints(cols, rows int, step float64) []geo.Point2D {
	points := make([]geo.Point2D, 0, cols*rows)

	for y := range rows {
		for x := range cols {
			points = append(points, geo.Point2D{X: float64(x) * step, Y: float64(y) * step})
		}
	}

	return points
}

func TestProbeReceiversSpreadOverTheGrid(t *testing.T) {
	t.Parallel()

	points := gridPoints(40, 40, 10)

	picked := ProbeReceivers(points, 16)
	if len(picked) != 16 {
		t.Fatalf("picked %d receivers on a square grid, want 16", len(picked))
	}

	if again := ProbeReceivers(points, 16); !slices.Equal(picked, again) {
		t.Fatalf("two selections differ: %v vs %v", picked, again)
	}

	// One per 4x4 stratum: every quarter of each axis is represented.
	seen := map[[2]int]bool{}
	for _, i := range picked {
		seen[[2]int{int(points[i].X) / 100, int(points[i].Y) / 100}] = true
	}

	if len(seen) != 16 {
		t.Fatalf("the 16 probe receivers cover %d of 16 strata: %v", len(seen), picked)
	}
}

func TestProbeReceiversTakesASmallSetWhole(t *testing.T) {
	t.Parallel()

	if got := ProbeReceivers(gridPoints(3, 3, 5), 16); !slices.Equal(got, []int{0, 1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatalf("got %v, want all nine", got)
	}
}

func TestProbeReceiversCutsALineAlongItsLength(t *testing.T) {
	t.Parallel()

	points := gridPoints(200, 1, 2)

	picked := ProbeReceivers(points, 16)
	if len(picked) != 16 {
		t.Fatalf("a line of receivers got %d strata, want 16", len(picked))
	}
}

func TestSampleOrderVisitsEveryUnitOnceAndSpreads(t *testing.T) {
	t.Parallel()

	for _, n := range []int{1, 2, 3, 7, 8, 100, 1000} {
		order := newSampleOrder(n)
		seen := make([]bool, n)

		for range n {
			i := order.take()
			if seen[i] {
				t.Fatalf("n=%d: unit %d visited twice", n, i)
			}

			seen[i] = true
		}
	}

	// The first four units of a thousand split it in quarters.
	order := newSampleOrder(1000)

	got := []int{order.take(), order.take(), order.take(), order.take()}
	if !slices.Equal(got, []int{0, 512, 256, 768}) {
		t.Fatalf("first four of 1000: %v", got)
	}
}

func TestMakespanReplaysTheChunks(t *testing.T) {
	t.Parallel()

	// 130 receivers on 12 workers: partition.Size gives chunks of ten, so
	// thirteen chunks, and one worker walks two of them.
	costs := make([]float64, 130)
	for i := range costs {
		costs[i] = 1
	}

	if got := makespan(costs, 12, SchedulePull); got != 20 {
		t.Fatalf("pull makespan %v, want 20", got)
	}

	if got := makespan(costs, 12, ScheduleSharded); got != 20 {
		t.Fatalf("sharded makespan %v, want 20", got)
	}

	if got := makespan(costs, 1, SchedulePull); got != 130 {
		t.Fatalf("one worker %v, want 130", got)
	}

	// A pull pool hands an expensive first chunk's neighbours to the other
	// workers; a sharded one cannot.
	uneven := make([]float64, 1000)
	for i := range uneven {
		uneven[i] = 1
	}

	uneven[0] = 500

	if pull, sharded := makespan(uneven, 2, SchedulePull), makespan(uneven, 2, ScheduleSharded); pull >= sharded {
		t.Fatalf("pull %v should beat sharded %v on an uneven load", pull, sharded)
	}
}

func TestFormatBand(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		band Band
		want string
	}{
		{Band{Low: 0.01, Estimate: 0.05, High: 0.2}, "< 1 s"},
		{Band{Low: 40.2, Estimate: 50, High: 69.1}, "≈ 40–70 s"},
		{Band{Low: 6.5 * 60, Estimate: 7 * 60, High: 8.2 * 60}, "≈ 6–9 min"},
		{Band{Low: 2.2 * 3600, Estimate: 2.5 * 3600, High: 2.9 * 3600}, "≈ 2–3 h"},
		{Band{Low: 30, Estimate: 30, High: 30}, "≈ 30 s"},
		{Band{Low: 40 * 60, Estimate: 2 * 3600, High: 4 * 3600}, "≈ 40–240 min"},
		{Band{Low: 50, Estimate: 300, High: 900}, "≈ 50–900 s"},
	} {
		if got := FormatBand(tc.band); got != tc.want {
			t.Fatalf("FormatBand(%+v) = %q, want %q", tc.band, got, tc.want)
		}
	}
}

// fakeKernel costs `spin` iterations of busy work per unit, and remembers the
// order its units were run in.
type fakeKernel struct {
	units    func(int) int
	spin     int
	calls    []int
	receiver int
	sink     float64
}

func (k *fakeKernel) Units(i int) int    { return k.units(i) }
func (k *fakeKernel) NewWalker() Walker  { return k }
func (k *fakeKernel) Begin(receiver int) { k.receiver = receiver }

func (k *fakeKernel) Unit(unit int) {
	k.calls = append(k.calls, k.receiver*1_000_000+unit)

	x := 0.0
	for i := range k.spin {
		x += math.Sqrt(float64(i))
	}

	k.sink = x
}

func TestRunWalksThePlan(t *testing.T) {
	t.Parallel()

	points := gridPoints(10, 10, 5)
	units := func(i int) int { return 50 + i }
	kernel := &fakeKernel{units: units, spin: 100}
	alt := &fakeKernel{units: func(int) int { return 5 }, spin: 100}

	estimate, err := Run(t.Context(), kernel, points, Options{
		Budget:              time.Minute,
		MaxUnitsPerReceiver: 8,
		Workers:             4,
		Alternatives: []AlternativeKernel{{
			Params: map[string]string{"k": "v"},
			Kernel: alt,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Each probe receiver visits its plan's units, rotated to its own offset.
	var want []int

	for k, p := range SamplePlan(points, units, 0, 8) {
		n := units(p.Receiver)
		for _, u := range p.Units {
			want = append(want, p.Receiver*1_000_000+(u+probeOffset(k, n))%n)
		}
	}

	if len(alt.calls) != len(SamplePlan(points, units, 0, 8))*5 {
		t.Fatalf("the alternative ran %d units, want five per probe receiver", len(alt.calls))
	}

	// Ahead of the plan, warmUp runs untimed units of the first probe
	// receiver.
	warm := len(kernel.calls) - len(want)
	if warm < 0 || !reflect.DeepEqual(kernel.calls[warm:], want) {
		t.Fatalf("units run %v, plan %v", kernel.calls, want)
	}

	for _, call := range kernel.calls[:warm] {
		if call/1_000_000 != want[0]/1_000_000 {
			t.Fatalf("warm-up ran a unit of receiver %d, not the first probe receiver's", call/1_000_000)
		}
	}

	if estimate.Inputs.Units != int64(100*50+99*100/2) {
		t.Fatalf("total units %d", estimate.Inputs.Units)
	}

	cpu := estimate.CPUSeconds
	if !(cpu.Low > 0 && cpu.Low < cpu.Estimate && cpu.Estimate < cpu.High) {
		t.Fatalf("cpu band is not ordered: %+v", cpu)
	}

	wall := estimate.WallSeconds
	if !(wall.Low < wall.Estimate && wall.Estimate < wall.High && wall.Estimate < cpu.Estimate) {
		t.Fatalf("wall band %+v against cpu %+v", wall, cpu)
	}

	if len(estimate.Alts) != 1 || estimate.Alts[0].Units != 500 || estimate.Alts[0].CPUSeconds.Estimate >= cpu.Estimate {
		t.Fatalf("alternative %+v", estimate.Alts)
	}
}

func TestRunStopsAtTheBudget(t *testing.T) {
	t.Parallel()

	kernel := &fakeKernel{units: func(int) int { return 1 << 20 }, spin: 2000}

	startedAt := time.Now()

	estimate, err := Run(t.Context(), kernel, gridPoints(10, 10, 5), Options{Budget: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	if elapsed := time.Since(startedAt); elapsed > 2*time.Second {
		t.Fatalf("a 50 ms budget took %v", elapsed)
	}

	if !estimate.Probe.BudgetExhausted || estimate.Probe.MeasuredReceivers == 0 {
		t.Fatalf("probe report %+v", estimate.Probe)
	}
}

func TestRunHonoursCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := Run(ctx, &fakeKernel{units: func(int) int { return 10 }}, gridPoints(2, 2, 1), Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestUnitCostIsExactWhenEverythingWasSampled(t *testing.T) {
	t.Parallel()

	costs := estimateUnitCosts([]receiverProbe{{units: 2, batches: []batch{{1, 0.1}, {1, 0.3}}}})
	if math.Abs(costs.mean[0]-0.2) > 1e-12 || costs.stderr[0] != 0 {
		t.Fatalf("mean %v stderr %v, want 0.2 and 0", costs.mean[0], costs.stderr[0])
	}
}

// A receiver that sampled little leans on the pool; one that sampled much
// keeps its own mean.
func TestUnitCostShrinksTowardsThePool(t *testing.T) {
	t.Parallel()

	few := receiverProbe{units: 1000, batches: []batch{{1, 1.0}}}
	many := receiverProbe{units: 100_000, batches: make([]batch, 0, 1000)}

	for range 1000 {
		many.batches = append(many.batches, batch{4, 0.004})
	}

	costs := estimateUnitCosts([]receiverProbe{few, many})

	if costs.mean[0] > 0.1 {
		t.Fatalf("one sampled unit at 1 s should be pulled towards the pool's 1 ms, got %v", costs.mean[0])
	}

	if math.Abs(costs.mean[1]-0.001) > 1e-4 {
		t.Fatalf("4000 sampled units should keep their own mean, got %v", costs.mean[1])
	}
}
