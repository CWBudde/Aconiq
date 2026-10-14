package iso9613

import (
	"fmt"
	"math"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/runestimate"
)

// probeScene is a small scene with everything a path can read: sources with
// and without an octave-band spectrum, with a directivity correction, a row of
// barriers that screens some paths and not others, two ground zones and a
// non-zero C_0, so C_met takes a value of its own on the far paths.
func probeScene(sourceCount int) ([]PointSource, PropagationConfig) {
	spectrum := BandLevels{88, 92, 95, 97, 96, 93, 89, 84}

	sources := make([]PointSource, 0, sourceCount)

	for i := range sourceCount {
		source := PointSource{
			ID:                      fmt.Sprintf("s-%d", i),
			Point:                   geo.Point2D{X: float64(i%10) * 37, Y: -40 - float64(i/10)*23},
			SourceHeightM:           0.5 + float64(i%4),
			SoundPowerLevelDB:       95 + float64(i%7),
			DirectivityCorrectionDB: float64(i%3) - 1,
		}

		if i%2 == 0 {
			bands := spectrum
			bands[i%NumBands] += 3
			source.OctaveBandLevels = &bands
		}

		sources = append(sources, source)
	}

	cfg := DefaultPropagationConfig()
	cfg.C0 = 2
	cfg.GroundFactor = 0.3

	for i := range 6 {
		x := float64(i)*60 - 10
		cfg.Barriers = append(cfg.Barriers, Barrier{
			ID:       fmt.Sprintf("b-%d", i),
			Geometry: []geo.Point2D{{X: x, Y: 15}, {X: x + 35, Y: 18}},
			HeightM:  3 + float64(i%3),
		})
	}

	cfg.GroundZones = []GroundZone{
		{ID: "field", Polygon: [][]geo.Point2D{{{X: -50, Y: -400}, {X: 200, Y: -400}, {X: 200, Y: 0}, {X: -50, Y: 0}, {X: -50, Y: -400}}}, GroundFactor: 1},
		{ID: "yard", Polygon: [][]geo.Point2D{{{X: 100, Y: 20}, {X: 400, Y: 20}, {X: 400, Y: 300}, {X: 100, Y: 300}, {X: 100, Y: 20}}}, GroundFactor: 0},
	}

	return sources, cfg
}

func probeReceivers(side int) []geo.PointReceiver {
	receivers := make([]geo.PointReceiver, 0, side*side)

	for iy := range side {
		for ix := range side {
			receivers = append(receivers, geo.PointReceiver{
				ID:      fmt.Sprintf("r-%d-%d", ix, iy),
				Point:   geo.Point2D{X: float64(ix) * 360 / float64(side), Y: 30 + float64(iy)*240/float64(side)},
				HeightM: 4 + float64(ix%2),
			})
		}
	}

	return receivers
}

// TestProbeUnitsAreTheWalksPaths pins the probe's units to the walk's paths in
// the strongest way available: adding every unit's contribution, in unit
// order, has to give the walk's levels to the bit. That holds only if the
// probe visits the same source→receiver paths as the walk, each exactly once,
// in the walk's order, and adds both of a path's summands — the downwind
// ratio and the C_met-corrected one — exactly as the walk adds them.
func TestProbeUnitsAreTheWalksPaths(t *testing.T) {
	t.Parallel()

	sources, cfg := probeScene(40)
	receivers := probeReceivers(5)

	kernel := newEstimateKernel(receivers, sources, cfg)
	walker := kernel.newWalker()

	for i, r := range receivers {
		want, err := ComputeReceiverIndicators(r, sources, cfg)
		if err != nil {
			t.Fatal(err)
		}

		walker.Begin(i)
		walker.sums = pathSums{}

		units := kernel.Units(i)
		if units != len(sources) {
			t.Fatalf("receiver %s: %d units, the walk visits %d paths", r.ID, units, len(sources))
		}

		for u := range units {
			walker.addUnit(u)
		}

		got := ReceiverIndicators{LpAeqDW: levelFromPressureRatio(walker.sums.dw), LpAeqLT: levelFromPressureRatio(walker.sums.lt)}

		if got != want {
			t.Fatalf("receiver %s: %d units sum to %+v, the walk computes %+v", r.ID, units, got, want)
		}
	}
}

// TestEstimateSamplesDeterministically holds the budget out of the way and
// the per-receiver count fixed, so the amount of work is as fixed as its
// selection, and asks for the same estimate twice.
func TestEstimateSamplesDeterministically(t *testing.T) {
	t.Parallel()

	sources, cfg := probeScene(40)
	receivers := probeReceivers(5)
	opts := runestimate.Options{
		Budget:              time.Minute,
		MaxProbeReceivers:   4,
		MaxUnitsPerReceiver: 24,
		Workers:             1,
	}

	first, err := Estimate(t.Context(), receivers, sources, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}

	second, err := Estimate(t.Context(), receivers, sources, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}

	kernel := newEstimateKernel(receivers, sources, cfg)
	points := make([]geo.Point2D, len(receivers))

	for i, r := range receivers {
		points[i] = r.Point
	}

	plan := runestimate.SamplePlan(points, kernel.Units, 4, 24)
	if again := runestimate.SamplePlan(points, kernel.Units, 4, 24); !reflect.DeepEqual(plan, again) {
		t.Fatalf("sample plan differs between two calls:\n%v\n%v", plan, again)
	}

	wantTimed := 0
	for _, p := range plan {
		wantTimed += len(p.Units)
	}

	if first.Probe.UnitsTimed != wantTimed || second.Probe.UnitsTimed != wantTimed {
		t.Fatalf("timed %d then %d units, the plan holds %d", first.Probe.UnitsTimed, second.Probe.UnitsTimed, wantTimed)
	}

	if !reflect.DeepEqual(first.Inputs, second.Inputs) || first.Probe.MeasuredReceivers != len(plan) {
		t.Fatalf("inputs differ or not every probe receiver measured: %+v vs %+v", first.Inputs, second.Inputs)
	}

	if first.StandardID != StandardID || !first.Available {
		t.Fatalf("estimate for %q, available %v", first.StandardID, first.Available)
	}

	if want := int64(len(receivers) * len(sources)); first.Inputs.Units != want {
		t.Fatalf("%d units, want one per source→receiver path (%d)", first.Inputs.Units, want)
	}

	wantCounts := map[string]int{"point_sources": 40, "barriers": 6, "ground_zones": 2}
	if !reflect.DeepEqual(first.Inputs.Counts, wantCounts) {
		t.Fatalf("counts %v, want %v", first.Inputs.Counts, wantCounts)
	}

	// The run prepares nothing up front, so neither does the estimate claim to.
	if first.Probe.PrepareSeconds != 0 {
		t.Fatalf("an ISO 9613-2 estimate reports %v s of scene preparation", first.Probe.PrepareSeconds)
	}

	if len(first.Alts) != 0 {
		t.Fatalf("an ISO 9613-2 estimate offers %+v", first.Alts)
	}
}

// TestEstimateRefusesWhatTheRunRefuses keeps the estimate from answering a
// question the run would refuse, in the run's own words: a broken receiver
// past the first, a broken source, a broken config and no receivers at all.
func TestEstimateRefusesWhatTheRunRefuses(t *testing.T) {
	t.Parallel()

	cases := map[string]func([]geo.PointReceiver, []PointSource, PropagationConfig) ([]geo.PointReceiver, []PointSource, PropagationConfig){
		"receiver without id": func(r []geo.PointReceiver, s []PointSource, c PropagationConfig) ([]geo.PointReceiver, []PointSource, PropagationConfig) {
			r[3].ID = ""

			return r, s, c
		},
		"receiver not finite": func(r []geo.PointReceiver, s []PointSource, c PropagationConfig) ([]geo.PointReceiver, []PointSource, PropagationConfig) {
			r[7].Point.X = math.NaN()

			return r, s, c
		},
		"source not finite": func(r []geo.PointReceiver, s []PointSource, c PropagationConfig) ([]geo.PointReceiver, []PointSource, PropagationConfig) {
			s[5].SoundPowerLevelDB = math.Inf(1)

			return r, s, c
		},
		"no sources": func(r []geo.PointReceiver, _ []PointSource, c PropagationConfig) ([]geo.PointReceiver, []PointSource, PropagationConfig) {
			return r, nil, c
		},
		"config out of range": func(r []geo.PointReceiver, s []PointSource, c PropagationConfig) ([]geo.PointReceiver, []PointSource, PropagationConfig) {
			c.GroundFactor = 2

			return r, s, c
		},
		"no receivers": func(_ []geo.PointReceiver, s []PointSource, c PropagationConfig) ([]geo.PointReceiver, []PointSource, PropagationConfig) {
			return nil, s, c
		},
	}

	for name, breakScene := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sources, cfg := probeScene(12)
			receivers, sources, cfg := breakScene(probeReceivers(4), sources, cfg)

			_, want := ComputeReceiverOutputs(receivers, sources, cfg)

			_, err := Estimate(t.Context(), receivers, sources, cfg, runestimate.Options{Budget: time.Second})
			if err == nil || want == nil || err.Error() != want.Error() {
				t.Fatalf("estimate refused with %v, the run with %v", err, want)
			}
		})
	}
}

// TestEstimateIsWithinAFactorOfTheRun runs the real walk over the scene and
// holds the estimate to it, within a factor of three either way.
//
// Both sides are read on the clock the probe times units on: the calling
// thread's CPU time where the platform has one (runestimate.ThreadCPUTime),
// the wall clock otherwise — see the RLS-19 test of the same name for why
// the wall clock is not trusted for this under a loaded `go test ./...`.
func TestEstimateIsWithinAFactorOfTheRun(t *testing.T) {
	if testing.Short() {
		t.Skip("times a run")
	}

	sources, cfg := probeScene(400)

	for i := range 60 {
		x := float64(i%12)*30 - 10
		y := 20 + float64(i/12)*40
		cfg.Barriers = append(cfg.Barriers, Barrier{
			ID:       fmt.Sprintf("row-%d", i),
			Geometry: []geo.Point2D{{X: x, Y: y}, {X: x + 20, Y: y + 2}},
			HeightM:  6,
		})
	}

	receivers := probeReceivers(20)

	estimate, err := Estimate(t.Context(), receivers, sources, cfg, runestimate.Options{
		Budget:  150 * time.Millisecond,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The walk runs on this goroutine, so with the thread held its CPU time
	// is the run's.
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	read := func() time.Duration {
		if cpu, ok := runestimate.ThreadCPUTime(); ok {
			return cpu
		}

		return time.Duration(time.Now().UnixNano())
	}

	startedAt := read()

	_, err = ComputeReceiverOutputs(receivers, sources, cfg)
	if err != nil {
		t.Fatal(err)
	}

	measured := (read() - startedAt).Seconds()

	// One worker and no preparation, so the estimate's compute is the run.
	estimated := estimate.CPUSeconds.Estimate + estimate.Probe.PrepareSeconds

	if estimated < measured/3 || estimated > measured*3 {
		t.Fatalf("estimated %.3fs (cpu band %.3f–%.3f, clock %s), the run took %.3fs",
			estimated, estimate.CPUSeconds.Low, estimate.CPUSeconds.High, estimate.Probe.Clock, measured)
	}

	t.Logf("estimated %.3fs (cpu band %.3f–%.3f, %d units timed, clock %s), measured %.3fs",
		estimated, estimate.CPUSeconds.Low, estimate.CPUSeconds.High, estimate.Probe.UnitsTimed,
		estimate.Probe.Clock, measured)
}
