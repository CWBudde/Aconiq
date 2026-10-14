package runestimate

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/aconiq/backend/internal/geo"
)

// DefaultBudget is how long a probe may take when the caller does not say.
//
// It bounds the timed probe and everything optional, checked between units;
// see Run for the three things that can still run past it. On the scenes
// where an estimate matters most a single receiver takes minutes, which is
// why the probe samples units and never whole receivers.
const DefaultBudget = 3 * time.Second

// Kernel is what a standard implements to be estimated.
type Kernel interface {
	// Units is how many work units the run's walk visits for one receiver,
	// counted by the same logic the walk uses to visit them. It must be cheap
	// next to walking them: the probe calls it once for every receiver of the
	// run.
	Units(receiver int) int

	// NewWalker returns a walker with the per-walk state a run's worker
	// starts with.
	NewWalker() Walker
}

// Walker computes single units of a run and discards what they produce.
type Walker interface {
	// Begin does what the walk does once per receiver before its first unit.
	// The probe times it as the receiver's one-off cost.
	Begin(receiver int)

	// Unit runs unit `unit` of the receiver last begun, exactly as the run's
	// walk would run it.
	Unit(unit int)
}

// AlternativeKernel is a variant of the run the estimate also costs: the
// same receivers under changed parameters, walked by its own kernel and
// measured by its own probe.
//
// It is measured rather than scaled from the run's own probe because a
// parameter that changes which units there are changes what a unit costs:
// distance-scaled Teilstücke keep the near pairs and merge the far ones, and
// on a building-dense scene a near pair admits several times the mirrored
// paths a far one does.
//
// NewKernel builds the variant's kernel, and is called only once the run's
// own probe is done and only if the budget has time left for the variant:
// preparing it is part of what the budget bounds.
type AlternativeKernel struct {
	Params    map[string]string
	NewKernel func() (Kernel, error)
}

// Options shape one estimate.
type Options struct {
	// Budget is the probe's wall-clock budget; zero means DefaultBudget.
	Budget time.Duration

	// MaxProbeReceivers bounds the stratified sample; zero means
	// DefaultMaxProbeReceivers.
	MaxProbeReceivers int

	// MaxUnitsPerReceiver bounds how many units one probe receiver times;
	// zero leaves only the budget and the receiver's own unit count. Tests
	// set it to make the amount of work as fixed as its selection.
	MaxUnitsPerReceiver int

	// Workers is how many workers the run will compute on, and Schedule how
	// it hands them chunks.
	Workers  int
	Schedule Schedule

	// Prepare is how long preparing the scene took. It is added to the wall
	// time as it is: the run prepares the same scene once, on one thread.
	Prepare time.Duration

	// Alternatives are probed after the run itself, from alternativeShare
	// of the budget between them.
	Alternatives []AlternativeKernel
}

// Tuning. Each of these is a statement about what the probe cannot see or
// cannot afford, and is kept here, named, rather than folded into an
// expression.
const (
	// bandSigmas is how many standard errors the band spans on each side.
	bandSigmas = 2.0

	// systematicFactor widens every band for what sampling error does not
	// cover: clock frequency, the other processes on the machine, GC, a warm
	// or cold cache. A probe of a few seconds cannot measure any of them.
	systematicFactor = 1.15

	// unknownSpread is the relative error assumed where the probe measured
	// nothing to take one from: a run whose units all cost no measurable time.
	unknownSpread = 1.0

	// minUnitCV is the least coefficient of variation a unit's cost is
	// assumed to have; see unitSigma.
	minUnitCV = 1.0

	// shrinkUnits is how many sampled units a probe receiver needs before its
	// own mean counts as much as the pool's. A receiver that timed three
	// units on a scene whose units range over two orders of magnitude knows
	// little about its own average, and the other receivers' units are the
	// best evidence there is; one that timed thousands knows its own.
	shrinkUnits = 32.0

	// parallelEfficiency and worstParallelEfficiency are the fraction of a
	// single worker's speed each of several workers is assumed to keep. The
	// probe runs on one thread, alone; a run shares memory bandwidth and, on
	// a laptop or with SMT, shares cores, and a hybrid CPU puts some of its
	// workers on slower cores. The point estimate assumes the first, the
	// band's slow end the second.
	parallelEfficiency      = 0.8
	worstParallelEfficiency = 0.55

	// warmUpShare is the part of the budget warmUp spends before timing.
	warmUpShare = 0.1

	// alternativeShare is the part of the budget the alternatives get, when
	// there are any.
	alternativeShare = 0.4

	// minBatch is how long one timed batch of units has to take before its
	// size stops growing. Below it the clock's resolution — a few
	// microseconds in a browser — would be most of the measurement.
	minBatch = time.Millisecond

	// maxBatchUnits caps the batch size growth.
	maxBatchUnits = 1 << 16
)

// Run estimates a run.
//
// points are the run's receivers in the run's order; kernel walks them. The
// estimate it returns has no StandardID and no Counts — the caller knows
// those, and Run does not.
//
// # What the budget bounds
//
// The budget bounds the timed probe, checked between units, plus anything
// optional: an alternative is built and probed only while budget remains,
// and reported as unmeasured otherwise. Three things can still run past it,
// because an estimate cannot be had without them: counting the run's own
// units (one Units call per receiver, cancellable through ctx), the first
// probe receiver's Begin, and the one unit already running when the deadline
// passes. On a city-sized scene a Begin is a few tenths of a second, so the
// probe overruns its budget by about that much.
func Run(ctx context.Context, kernel Kernel, points []geo.Point2D, opts Options) (Estimate, error) {
	if len(points) == 0 {
		return Estimate{}, errors.New("estimate: at least one receiver is required")
	}

	budget := opts.Budget
	if budget <= 0 {
		budget = DefaultBudget
	}

	startedAt := time.Now()
	deadline := startedAt.Add(budget)
	probeOrder := ProbeReceivers(points, opts.MaxProbeReceivers)

	// Held for the whole probe, so the thread CPU clock reads this
	// goroutine's work and nothing another goroutine did on the same thread.
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	clock, onCPU := newUnitClock()

	units, total, _, err := countUnits(ctx, kernel, len(points), time.Time{})
	if err != nil {
		return Estimate{}, err
	}

	// One walker for the warm-up and the probe, so the first probe receiver
	// is begun once: on the RLS-19 kernel a Begin builds the receiver's mirror
	// table, and that is most of what a probe receiver costs on a dense scene.
	walker := kernel.NewWalker()
	firstSetup := warmUp(walker, clock, probeOrder[0], units[probeOrder[0]], startedAt.Add(time.Duration(float64(budget)*warmUpShare)))

	ownDeadline := deadline
	if len(opts.Alternatives) > 0 {
		ownDeadline = startedAt.Add(time.Duration(float64(budget) * (1 - alternativeShare)))
	}

	own, err := probeKernel(ctx, walker, units, points, probeOrder, probeLimits{
		deadline:    ownDeadline,
		perReceiver: opts.MaxUnitsPerReceiver,
		mustMeasure: true,
		firstSetup:  firstSetup,
		clock:       clock,
	})
	if err != nil {
		return Estimate{}, err
	}

	own.total = total

	estimate := extrapolate(points, own, opts)
	timed, exhausted := own.timed, own.exhausted

	for k, alt := range opts.Alternatives {
		measured, err := probeAlternative(ctx, alt, points, probeOrder, deadline, len(opts.Alternatives)-k, opts.MaxUnitsPerReceiver, clock)
		if err != nil {
			return Estimate{}, err
		}

		if len(measured.probes) == 0 {
			estimate.Alts = append(estimate.Alts, Alternative{Params: alt.Params, Measured: false})
			exhausted = true

			continue
		}

		estimate.Alts = append(estimate.Alts, costAlternative(alt.Params, points, measured, opts))
		timed += measured.timed
		exhausted = exhausted || measured.exhausted
	}

	estimate.Probe = &ProbeReport{
		ProbeReceivers:    len(probeOrder),
		MeasuredReceivers: len(own.probes),
		UnitsTimed:        timed,
		BudgetSeconds:     budget.Seconds(),
		ElapsedSeconds:    time.Since(startedAt).Seconds(),
		PrepareSeconds:    opts.Prepare.Seconds(),
		BudgetExhausted:   exhausted,
		Clock:             clockName(onCPU),
	}

	return estimate, nil
}

// probeAlternative builds and probes one alternative within an equal part of
// what is left of the budget — so one that overran does not starve the next
// of its whole share — or returns an empty measurement when nothing is left.
// Building the kernel and counting its units both happen inside that share,
// and a count that runs past it leaves the alternative unmeasured.
func probeAlternative(
	ctx context.Context,
	alt AlternativeKernel,
	points []geo.Point2D,
	probeOrder []int,
	deadline time.Time,
	remainingAlternatives int,
	perReceiver int,
	clock unitClock,
) (measurement, error) {
	now := time.Now()
	if !now.Before(deadline) {
		return measurement{}, nil
	}

	share := now.Add(deadline.Sub(now) / time.Duration(remainingAlternatives))

	kernel, err := alt.NewKernel()
	if err != nil {
		return measurement{}, err
	}

	units, total, counted, err := countUnits(ctx, kernel, len(points), share)
	if err != nil || !counted {
		return measurement{}, err
	}

	m, err := probeKernel(ctx, kernel.NewWalker(), units, points, probeOrder, probeLimits{
		deadline:    share,
		perReceiver: perReceiver,
		mustMeasure: false,
		firstSetup:  -1,
		clock:       clock,
	})
	m.total = total

	return m, err
}

// unitsCtxStride is how many receivers countUnits counts between two checks
// of its context and deadline.
const unitsCtxStride = 1024

// countUnits counts every receiver's units. It stops with ctx's error once
// ctx is done, and with counted false once a non-zero deadline has passed.
func countUnits(ctx context.Context, kernel Kernel, receivers int, deadline time.Time) ([]int, int64, bool, error) {
	units := make([]int, receivers)

	var total int64

	for i := range units {
		if i%unitsCtxStride == 0 {
			err := ctx.Err()
			if err != nil {
				return nil, 0, false, fmt.Errorf("estimate: %w", err)
			}

			if !deadline.IsZero() && !time.Now().Before(deadline) {
				return nil, 0, false, nil
			}
		}

		units[i] = kernel.Units(i)
		total += int64(units[i])
	}

	return units, total, true, nil
}

// warmUp begins the first probe receiver on walker and runs its units,
// untimed, until deadline. It returns what the Begin took, which the probe
// reports as that receiver's one-off cost: the probe begins it again on the
// same walker, and the second Begin is the cache hit the first one paid for.
//
// A run of minutes computes on a warm machine: the core is at its working
// clock, the scene's arrays are in cache, and the garbage the scene
// preparation left behind was collected long ago. A probe of a few seconds
// that starts timing straight away measures none of that — on a laptop the
// first tens of milliseconds can run an order of magnitude slower than the
// rest, and with sixteen receivers each sampling a handful of units, the
// first receiver's share of that would be extrapolated over a sixteenth of
// the grid. So the collection is forced and the core is warmed first, on the
// same walker the probe then times.
func warmUp(walker Walker, clock unitClock, receiver, units int, deadline time.Time) float64 {
	runtime.GC()

	if units == 0 {
		return -1
	}

	startedAt := clock()

	walker.Begin(receiver)

	setup := (clock() - startedAt).Seconds()
	order := newSampleOrder(units)

	for i := 0; i < units && time.Now().Before(deadline); i++ {
		walker.Unit(order.take())
	}

	return setup
}

// measurement is one kernel's probe: every receiver's unit count, and what
// the probe receivers measured.
type measurement struct {
	units     []int
	total     int64
	probes    []receiverProbe
	timed     int
	exhausted bool
}

// receiverProbe is what one probe receiver measured.
type receiverProbe struct {
	receiver int
	point    geo.Point2D
	units    int

	// batches are the timed batches of units.
	batches []batch

	// setup is the receiver's one-off cost, in seconds: what Begin took.
	setup float64
}

type batch struct {
	units   int
	seconds float64
}

func (p *receiverProbe) sampled() int {
	n := 0
	for _, b := range p.batches {
		n += b.units
	}

	return n
}

// probeLimits are the bounds of one kernel's probe.
type probeLimits struct {
	deadline    time.Time
	perReceiver int

	// mustMeasure measures the first probe receiver even past the deadline:
	// the run's own estimate needs one measurement, however late. An
	// alternative does not, and is reported unmeasured instead.
	mustMeasure bool

	// firstSetup is the first probe receiver's Begin as warmUp measured it,
	// or negative when nothing was warmed.
	firstSetup float64

	// clock times the units; see clock.go.
	clock unitClock
}

// probeKernel times the probe receivers' units until the deadline.
//
// Every probe receiver gets an equal part of what is left of the budget when
// its turn comes, so one that overran costs the next ones only their share.
// Past the deadline the remaining probe receivers are skipped — the nearest
// measured one stands in for them.
func probeKernel(
	ctx context.Context,
	walker Walker,
	units []int,
	points []geo.Point2D,
	probeOrder []int,
	limits probeLimits,
) (measurement, error) {
	m := measurement{units: units}

	for k, r := range probeOrder {
		err := ctx.Err()
		if err != nil {
			return measurement{}, fmt.Errorf("estimate: %w", err)
		}

		now := time.Now()

		if !now.Before(limits.deadline) && (len(m.probes) > 0 || !limits.mustMeasure) {
			m.exhausted = true

			break
		}

		sliceEnd := now.Add(limits.deadline.Sub(now) / time.Duration(len(probeOrder)-k))

		probe, cutShort := measureReceiver(walker, limits.clock, r, units[r], probeOffset(k, units[r]), sliceEnd, limits.perReceiver)
		probe.point = points[r]

		if k == 0 && limits.firstSetup >= 0 {
			probe.setup = limits.firstSetup
		}

		m.exhausted = m.exhausted || cutShort
		m.timed += probe.sampled()
		m.probes = append(m.probes, probe)
	}

	return m, nil
}

// probeOffset is where the k-th probe receiver starts in its own units: the
// k-th point of a van der Corput sequence over them. Every receiver visits
// its units in the same spread order, but from a different place, so a
// budget that affords each receiver a single unit has still timed sixteen
// parts of the scene rather than the first Teilstück sixteen times.
func probeOffset(k, units int) int {
	if units <= 1 {
		return 0
	}

	return int(float64(units) * vanDerCorput(k))
}

// measureReceiver times one probe receiver: its Begin, then its units in
// sample order from offset, until its slice of the budget runs out, it has
// sampled perReceiverCap of them, or it has run them all. cutShort reports
// the first of those.
func measureReceiver(walker Walker, clock unitClock, receiver, units, offset int, sliceEnd time.Time, perReceiverCap int) (receiverProbe, bool) {
	probe := receiverProbe{receiver: receiver, units: units}
	if units == 0 {
		return probe, false
	}

	want := units
	if perReceiverCap > 0 {
		want = min(want, perReceiverCap)
	}

	startedAt := clock()

	walker.Begin(receiver)

	probe.setup = (clock() - startedAt).Seconds()

	order := newSampleOrder(units)
	batchSize, sampled := 1, 0

	for sampled < want {
		if len(probe.batches) > 0 && !time.Now().Before(sliceEnd) {
			return probe, true
		}

		size := min(batchSize, want-sampled)
		batchStart := clock()

		for range size {
			walker.Unit((order.take() + offset) % units)
		}

		elapsed := clock() - batchStart
		probe.batches = append(probe.batches, batch{units: size, seconds: elapsed.Seconds()})
		sampled += size

		if elapsed < minBatch && batchSize < maxBatchUnits {
			batchSize *= 2
		}
	}

	return probe, false
}

// clockName is how ProbeReport names the clock the units were timed on.
func clockName(onCPU bool) string {
	if onCPU {
		return ClockThreadCPU
	}

	return ClockWall
}
