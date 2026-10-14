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
// It is a wall-clock bound on the probe, checked between units: a unit already
// running finishes, so the probe can overrun by one unit's cost. On the
// scenes where an estimate matters most a single receiver takes minutes,
// which is why the probe samples units and never whole receivers.
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
type AlternativeKernel struct {
	Params map[string]string
	Kernel Kernel
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
func Run(ctx context.Context, kernel Kernel, points []geo.Point2D, opts Options) (Estimate, error) {
	if len(points) == 0 {
		return Estimate{}, errors.New("estimate: at least one receiver is required")
	}

	budget := opts.Budget
	if budget <= 0 {
		budget = DefaultBudget
	}

	startedAt := time.Now()
	probeOrder := ProbeReceivers(points, opts.MaxProbeReceivers)

	warmUp(kernel, probeOrder[0], startedAt.Add(time.Duration(float64(budget)*warmUpShare)))

	ownBudget := budget
	if len(opts.Alternatives) > 0 {
		ownBudget = time.Duration(float64(budget) * (1 - alternativeShare))
	}

	own, err := probeKernel(ctx, kernel, points, probeOrder, startedAt.Add(ownBudget), opts.MaxUnitsPerReceiver)
	if err != nil {
		return Estimate{}, err
	}

	estimate := extrapolate(points, own, opts)
	timed, exhausted := own.timed, own.exhausted

	for k, alt := range opts.Alternatives {
		// Each alternative gets an equal part of what is left, so one that
		// overran does not starve the next of its whole share.
		remaining := budget - time.Since(startedAt)
		deadline := time.Now().Add(remaining / time.Duration(len(opts.Alternatives)-k))

		measured, err := probeKernel(ctx, alt.Kernel, points, probeOrder, deadline, opts.MaxUnitsPerReceiver)
		if err != nil {
			return Estimate{}, err
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
	}

	return estimate, nil
}

// warmUp runs units of the first probe receiver, untimed, until deadline.
//
// A run of minutes computes on a warm machine: the core is at its working
// clock, the scene's arrays are in cache, and the garbage the scene
// preparation left behind was collected long ago. A probe of a few seconds
// that starts timing straight away measures none of that — on a laptop the
// first tens of milliseconds can run an order of magnitude slower than the
// rest, and with sixteen receivers each sampling a handful of units, the
// first receiver's share of that would be extrapolated over a sixteenth of
// the grid. So the collection is forced and the core is warmed first, on the
// same kernel the probe then times.
func warmUp(kernel Kernel, receiver int, deadline time.Time) {
	runtime.GC()

	units := kernel.Units(receiver)
	if units == 0 {
		return
	}

	walker := kernel.NewWalker()
	walker.Begin(receiver)

	order := newSampleOrder(units)

	for i := 0; i < units && time.Now().Before(deadline); i++ {
		walker.Unit(order.take())
	}
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

// probeKernel counts every receiver's units and times the probe receivers'
// until deadline.
//
// Every probe receiver gets an equal part of what is left of the budget when
// its turn comes, so one that overran costs the next ones only their share.
// Past the deadline the remaining probe receivers are skipped — the nearest
// measured one stands in for them — except that the first is always
// measured: an estimate needs one measurement, however late.
func probeKernel(
	ctx context.Context,
	kernel Kernel,
	points []geo.Point2D,
	probeOrder []int,
	deadline time.Time,
	perReceiverCap int,
) (measurement, error) {
	m := measurement{units: make([]int, len(points))}

	for i := range points {
		m.units[i] = kernel.Units(i)
		m.total += int64(m.units[i])
	}

	walker := kernel.NewWalker()

	for k, r := range probeOrder {
		err := ctx.Err()
		if err != nil {
			return measurement{}, fmt.Errorf("estimate: %w", err)
		}

		now := time.Now()

		if !now.Before(deadline) && len(m.probes) > 0 {
			m.exhausted = true

			break
		}

		sliceEnd := now.Add(deadline.Sub(now) / time.Duration(len(probeOrder)-k))

		probe, cutShort := measureReceiver(walker, r, m.units[r], probeOffset(k, m.units[r]), sliceEnd, perReceiverCap)
		probe.point = points[r]

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
func measureReceiver(walker Walker, receiver, units, offset int, sliceEnd time.Time, perReceiverCap int) (receiverProbe, bool) {
	probe := receiverProbe{receiver: receiver, units: units}
	if units == 0 {
		return probe, false
	}

	want := units
	if perReceiverCap > 0 {
		want = min(want, perReceiverCap)
	}

	startedAt := time.Now()

	walker.Begin(receiver)

	probe.setup = time.Since(startedAt).Seconds()

	order := newSampleOrder(units)
	batchSize, sampled := 1, 0

	for sampled < want {
		if len(probe.batches) > 0 && !time.Now().Before(sliceEnd) {
			return probe, true
		}

		size := min(batchSize, want-sampled)
		batchStart := time.Now()

		for range size {
			walker.Unit((order.take() + offset) % units)
		}

		elapsed := time.Since(batchStart)
		probe.batches = append(probe.batches, batch{units: size, seconds: elapsed.Seconds()})
		sampled += size

		if elapsed < minBatch && batchSize < maxBatchUnits {
			batchSize *= 2
		}
	}

	return probe, false
}
