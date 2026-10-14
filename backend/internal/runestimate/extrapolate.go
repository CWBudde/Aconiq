package runestimate

import (
	"math"

	"github.com/aconiq/backend/internal/geo"
)

// unitCosts is what a measurement says a unit costs at each probe receiver.
type unitCosts struct {
	// mean is the per-unit cost each probe receiver stands for, in seconds,
	// and stderr its standard error.
	mean   []float64
	stderr []float64
}

// estimateUnitCosts turns the batches into a per-unit cost at every probe
// receiver.
//
// The variance comes from batches of any size: a batch mean over k units
// varies as σ²/k, so k·(x̄_b − μ)² estimates σ² whatever k was, and batches of
// mixed sizes pool without a common size to wait for.
//
// A receiver's own mean is shrunk towards the pool's in proportion to how
// little it sampled (shrinkUnits): on an expensive scene the budget affords a
// receiver one or two units, and those say less about its average than all
// the probe receivers' units together. On a cheap scene a receiver samples
// thousands and keeps its own mean. The finite-population factor takes a
// receiver's own error to zero when it sampled every unit, which is exact.
func estimateUnitCosts(probes []receiverProbe) unitCosts {
	var pooledSeconds, pooledUnits float64

	for i := range probes {
		for _, b := range probes[i].batches {
			pooledSeconds += b.seconds
			pooledUnits += float64(b.units)
		}
	}

	costs := unitCosts{mean: make([]float64, len(probes)), stderr: make([]float64, len(probes))}
	if pooledUnits == 0 {
		return costs
	}

	pooledMean := pooledSeconds / pooledUnits
	pooledSigma := unitSigma(allBatches(probes), pooledMean)
	pooledErr := pooledSigma / math.Sqrt(pooledUnits)

	for i := range probes {
		p := &probes[i]

		sampled := float64(p.sampled())
		if sampled == 0 {
			costs.mean[i], costs.stderr[i] = pooledMean, pooledSigma

			continue
		}

		own := 0.0
		for _, b := range p.batches {
			own += b.seconds
		}

		own /= sampled

		sigma := pooledSigma
		if len(p.batches) >= 2 {
			sigma = unitSigma(p.batches, own)
		}

		finite := math.Sqrt(math.Max(0, 1-sampled/float64(p.units)))
		ownErr := sigma / math.Sqrt(sampled) * finite

		weight := sampled / (sampled + shrinkUnits)
		if finite == 0 {
			weight = 1
		}

		costs.mean[i] = weight*own + (1-weight)*pooledMean
		costs.stderr[i] = math.Hypot(weight*ownErr, (1-weight)*pooledErr)
	}

	return costs
}

func allBatches(probes []receiverProbe) []batch {
	var all []batch
	for i := range probes {
		all = append(all, probes[i].batches...)
	}

	return all
}

// unitSigma is the per-unit standard deviation the batches imply around mean,
// and never less than mean·minUnitCV.
//
// The floor is what keeps a small sample honest. What a unit costs is
// heavy-tailed — a Teilstück beside a courtyard admits hundreds of mirrored
// paths, one across an open field none — and a handful of units, which is
// all the budget affords on a dense scene, has usually seen none of the
// tail and reports a spread far smaller than the population's. On a cheap
// scene the probe samples thousands of units and the floor costs nothing.
func unitSigma(batches []batch, mean float64) float64 {
	floor := mean * minUnitCV
	if len(batches) < 2 {
		return floor
	}

	sum := 0.0

	for _, b := range batches {
		d := b.seconds/float64(b.units) - mean
		sum += float64(b.units) * d * d
	}

	return math.Max(math.Sqrt(sum/float64(len(batches)-1)), floor)
}

// extrapolate turns one measurement into the whole run.
//
// Every receiver borrows the per-unit cost and the one-off cost of the probe
// receiver nearest to it — the probe receivers are the strata's
// representatives, so "nearest" is the stratum it fell in, and it stays
// well-defined for a stratum whose probe the budget never reached. The unit
// count is the receiver's own, counted, not borrowed: what is interpolated is
// only what a unit costs there.
func extrapolate(points []geo.Point2D, m measurement, opts Options) Estimate {
	cpuBand, wall, costs := costRun(points, m, opts)

	workers := max(opts.Workers, 1)
	perReceiver := cpuBand.scaled(1 / float64(len(points)))

	cpu := 0.0
	for _, c := range costs {
		cpu += c
	}

	return Estimate{
		Available:             true,
		WallSeconds:           &wall,
		CPUSeconds:            &cpuBand,
		CPUPerReceiverSeconds: &perReceiver,
		Parallelism:           parallelism(cpu, wall.Estimate-opts.Prepare.Seconds()),
		Inputs: &Inputs{
			Receivers: len(points),
			Workers:   workers,
			Units:     m.total,
		},
	}
}

// costAlternative prices an alternative's measurement on the run's workers and
// schedule, with the preparation the alternative reported for its own scene
// in place of the run's.
func costAlternative(params map[string]string, points []geo.Point2D, m measurement, opts Options) Alternative {
	opts.Prepare = m.prepare

	cpu, wall, _ := costRun(points, m, opts)
	prepare := m.prepare.Seconds()

	return Alternative{
		Params:         params,
		Measured:       true,
		Units:          m.total,
		CPUSeconds:     &cpu,
		WallSeconds:    &wall,
		PrepareSeconds: &prepare,
	}
}

// costRun is the CPU band, the wall band and the per-receiver point costs of
// one measurement.
func costRun(points []geo.Point2D, m measurement, opts Options) (Band, Band, []float64) {
	owner := nearestProbe(points, m.probes)
	unit := estimateUnitCosts(m.probes)

	costs := make([]float64, len(points))
	represented := make([]float64, len(m.probes))

	cpu := 0.0

	for i, n := range m.units {
		if n == 0 || len(m.probes) == 0 {
			continue
		}

		k := owner[i]
		costs[i] = m.probes[k].setup + unit.mean[k]*float64(n)
		represented[k] += float64(n)
		cpu += costs[i]
	}

	cpuBand := bandAround(cpu, relativeError(unit, represented, cpu, len(points) == len(m.probes)))

	return cpuBand, wallBand(costs, cpuBand, max(opts.Workers, 1), opts), costs
}

func nearestProbe(points []geo.Point2D, probes []receiverProbe) []int {
	owner := make([]int, len(points))

	for i, p := range points {
		best, bestDist := 0, math.Inf(1)

		for k := range probes {
			d := squaredDistance(p, probes[k].point)
			if d < bestDist {
				best, bestDist = k, d
			}
		}

		owner[i] = best
	}

	return owner
}

// relativeError combines the two errors the probe can see.
//
// Sampling: every unit a probe receiver stands for carries the standard
// error of that receiver's unit cost. Representativeness: a probe receiver
// stands for its stratum, and how far the strata differ from one another is
// the best available guess at how far a stratum's receivers differ from its
// probe. The spread between the probe receivers' unit costs, over √probes,
// is the standard error of the run's average unit cost. exhaustive says every
// receiver was probed, which leaves nothing to extrapolate between them.
func relativeError(unit unitCosts, represented []float64, cpu float64, exhaustive bool) float64 {
	if cpu <= 0 {
		return unknownSpread
	}

	variance := 0.0

	for k, n := range represented {
		e := n * unit.stderr[k]
		variance += e * e
	}

	sampling := math.Sqrt(variance) / cpu

	spread := 0.0

	if !exhaustive && len(unit.mean) >= 2 {
		average := 0.0
		for _, m := range unit.mean {
			average += m
		}

		average /= float64(len(unit.mean))
		if average > 0 {
			spread = sampleStdDev(unit.mean) / average / math.Sqrt(float64(len(unit.mean)))
		}
	}

	return math.Hypot(sampling, spread)
}

func sampleStdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	mean := 0.0
	for _, v := range values {
		mean += v
	}

	mean /= float64(len(values))

	sum := 0.0

	for _, v := range values {
		d := v - mean
		sum += d * d
	}

	return math.Sqrt(sum / float64(len(values)-1))
}

// bandAround is a band of bandSigmas relative errors on each side of point,
// multiplicative so that the low end never reaches zero, and widened by
// systematicFactor.
func bandAround(point, rel float64) Band {
	widen := (1 + bandSigmas*rel) * systematicFactor

	return Band{Low: point / widen, Estimate: point, High: point * widen}
}

// wallBand is the elapsed time: the chunk schedule over the per-receiver
// costs, at the parallel efficiency each end of the band assumes, plus the
// scene preparation.
func wallBand(costs []float64, cpu Band, workers int, opts Options) Band {
	span := makespan(costs, workers, opts.Schedule)
	prepare := opts.Prepare.Seconds()

	if cpu.Estimate <= 0 {
		return Band{Low: prepare, Estimate: prepare, High: prepare}
	}

	pointEfficiency, worstEfficiency := 1.0, 1.0
	if workers > 1 && len(costs) > 1 {
		pointEfficiency, worstEfficiency = parallelEfficiency, worstParallelEfficiency
	}

	return Band{
		Low:      prepare + span*cpu.Low/cpu.Estimate,
		Estimate: prepare + span/pointEfficiency,
		High:     prepare + span*cpu.High/cpu.Estimate/worstEfficiency,
	}
}

func parallelism(cpu, wall float64) float64 {
	if wall <= 0 || cpu <= 0 {
		return 1
	}

	return cpu / wall
}
