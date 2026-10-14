package schall03

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

// probeHillside is the ground under the probe scenes: a slope rising away
// from the track with a ripple along it, so every path's Gl. 15 ground term
// is a real sample and not the flat-plane zero.
func probeHillside() profileTerrain {
	return profileTerrain{
		bounds: [4]float64{-5000, -5000, 5000, 5000},
		z:      func(x, y float64) float64 { return 0.02*y + 3*math.Sin(x/100) },
	}
}

// probeNormativeScene is a small Anlage-2 scene with everything one
// receiver's walk can visit: an Eisenbahn line carrying three train types, a
// Straßenbahn line whose Haltestelle cuts it into three speed-zone parts, a
// row of facades on either side of the tracks, a screen, and a hillside under
// all of it. trackLengthM stretches the Eisenbahn line; side is the receiver
// grid's edge.
func probeNormativeScene(t *testing.T, trackLengthM float64, side int) ([]ReceiverInput, NormativeScene) {
	t.Helper()

	operations := make([]TrainOperation, 0, 3)

	for _, name := range []string{"ICE-1-Zug", "IC-Zug-E-Lok", "Gueterzug-E-Lok"} {
		op, err := NewTrainOperationFromZugart(name, 4, 2)
		if err != nil {
			t.Fatal(err)
		}

		operations = append(operations, *op)
	}

	half := trackLengthM / 2
	rail := TrackSegment{
		ID:              "rail",
		TrackCenterline: []geo.Point2D{{X: 200 - half, Y: -50}, {X: 200, Y: -45}, {X: 200 + half, Y: -50}},
		Fahrbahn:        FahrbahnartSchwellengleis,
		Surface:         SurfaceCondNone,
		StreckeMaxKPH:   160,
		Operations:      operations,
	}

	walls := make([]ReflectingWall, 0, 16)

	for i := range 8 {
		x := float64(i)*60 - 40
		walls = append(
			walls,
			ReflectingWall{A: geo.Point2D{X: x, Y: 25}, B: geo.Point2D{X: x + 30, Y: 25}, HeightM: 10, Surface: WallSurfaceHard},
			ReflectingWall{A: geo.Point2D{X: x, Y: -90}, B: geo.Point2D{X: x + 30, Y: -90}, HeightM: 12, Surface: WallSurfaceHard},
		)
	}

	hill := probeHillside()
	receivers := make([]ReceiverInput, 0, side*side)

	for iy := range side {
		for ix := range side {
			x := -50 + float64(ix)*500/float64(side)
			y := 40 + float64(iy)*150/float64(side)
			z, _ := hill.ElevationAt(x, y)
			receivers = append(receivers, ReceiverInput{
				ID:       fmt.Sprintf("r-%d-%d", ix, iy),
				Point:    geo.Point2D{X: x, Y: y},
				HeightM:  2 + float64(ix%3),
				TerrainZ: z,
			})
		}
	}

	return receivers, NormativeScene{
		Segments: []TrackSegment{rail, tramSegment(haltestelleAt(200))},
		Walls:    walls,
		Barriers: []BarrierSegment{{A: geo.Point2D{X: 50, Y: 12}, B: geo.Point2D{X: 180, Y: 12}, TopHeightM: 3}},
		Terrain:  hill,
	}
}

// probePreviewScene is a preview-chain scene over every source adjustment the
// chain has: slab track, a bridge, a curve, and plain ballast, with
// sourceCount tracks in all and a side×side receiver grid.
func probePreviewScene(sourceCount, side int) ([]geo.PointReceiver, []RailSource) {
	_, template := cancellationPreviewSources()
	sources := make([]RailSource, sourceCount)

	for i := range sources {
		source := template[0]
		source.ID = fmt.Sprintf("track-%d", i)
		y := float64(i) * 8
		source.TrackCenterline = []geo.Point2D{{X: -200, Y: y}, {X: 100, Y: y + 5}, {X: 400 + float64(i)*20, Y: y}}

		switch i % 4 {
		case 1:
			source.TrainClass = TrainClassFreight
			source.Infrastructure.TrackType = TrackTypeBallasted
			source.Infrastructure.TrackForm = TrackFormMainline
		case 2:
			source.Infrastructure.OnBridge = true
		case 3:
			source.Infrastructure.CurveRadiusM = 300
			source.Infrastructure.TractionType = TractionDiesel
		}

		sources[i] = source
	}

	receivers := make([]geo.PointReceiver, 0, side*side)

	for iy := range side {
		for ix := range side {
			receivers = append(receivers, geo.PointReceiver{
				ID:      fmt.Sprintf("r-%d-%d", ix, iy),
				Point:   geo.Point2D{X: -100 + float64(ix)*500/float64(side), Y: 60 + float64(iy)*200/float64(side)},
				HeightM: 4,
			})
		}
	}

	return receivers, sources
}

// TestNormativeProbeUnitsAreTheWalksLineSources pins the normative probe's
// units to the walk's line-source walks in the strongest way available:
// running every unit of a receiver, in unit order, into a day and a night sum
// has to give the walk's own two sums to the bit. That holds only if the
// probe visits the same (speed-zone part, operation, period) walks as the
// run, each exactly once, in the run's order, with the run's emission — in
// particular only if it reads the Haltestelle's three parts as the walk
// does.
func TestNormativeProbeUnitsAreTheWalksLineSources(t *testing.T) {
	t.Parallel()

	receivers, scene := probeNormativeScene(t, 600, 3)

	parts, err := prepareSpeedZoneParts(scene.Segments)
	if err != nil {
		t.Fatal(err)
	}

	// One part of three operations on the rail, three parts of one on the
	// tram, each walked by day and by night.
	kernel := newNormativeKernel(receivers, scene, parts)
	if got := kernel.Units(0); got != 2*(3+3) {
		t.Fatalf("a receiver walks %d units, want 12", got)
	}

	walker := kernel.newWalker()

	for i, r := range receivers {
		wantDay, wantNight, err := walkNormativeReceiver(t.Context(), r, scene)
		if err != nil {
			t.Fatal(err)
		}

		walker.Begin(i)
		walker.sums = periodSums{}

		units := kernel.Units(i)
		for u := range units {
			walker.addUnit(u)
		}

		gotDay, gotNight := walker.sums[periodDay].Sum(), walker.sums[periodNight].Sum()

		if math.Float64bits(gotDay) != math.Float64bits(wantDay) || math.Float64bits(gotNight) != math.Float64bits(wantNight) {
			t.Fatalf("receiver %s: %d units sum to day %v night %v, the walk to day %v night %v",
				r.ID, units, gotDay, gotNight, wantDay, wantNight)
		}

		if !(wantDay > 0) || !(wantNight > 0) {
			t.Fatalf("receiver %s collected no energy (day %v, night %v): the comparison proves nothing", r.ID, wantDay, wantNight)
		}
	}
}

// TestPreviewProbeUnitsAreTheWalksSources is the same pin for the preview
// chain, whose unit is one source: every unit's day and night spectrum, in
// unit order, has to be the walk's list of spectra to the bit, and summing
// them as the walk sums them has to give the walk's levels.
func TestPreviewProbeUnitsAreTheWalksSources(t *testing.T) {
	t.Parallel()

	receivers, sources := probePreviewScene(6, 3)
	cfg, pack := DefaultPropagationConfig(), BuiltinDataPack()

	kernel := newPreviewKernel(receivers, sources, cfg, pack)
	walker := kernel.newWalker()

	for i, r := range receivers {
		wantDay, wantNight, err := walkPreviewReceiver(t.Context(), r.Point, sources, cfg, pack)
		if err != nil {
			t.Fatal(err)
		}

		walker.Begin(i)
		walker.day, walker.night = nil, nil

		units := kernel.Units(i)
		if units != len(sources) {
			t.Fatalf("receiver %s walks %d units, want one per source (%d)", r.ID, units, len(sources))
		}

		for u := range units {
			walker.addUnit(u)
		}

		if !reflect.DeepEqual(walker.day, wantDay) || !reflect.DeepEqual(walker.night, wantNight) {
			t.Fatalf("receiver %s: the units' spectra differ from the walk's", r.ID)
		}

		want, err := ComputeReceiverPeriodLevelsWithDataPack(r.Point, sources, cfg, pack)
		if err != nil {
			t.Fatal(err)
		}

		got := PeriodLevels{LrDay: SumSpectra(walker.day).EnergeticTotal(), LrNight: SumSpectra(walker.night).EnergeticTotal()}
		if got != want {
			t.Fatalf("receiver %s: units sum to %+v, the walk computes %+v", r.ID, got, want)
		}
	}
}

// TestNormativeEstimateSamplesDeterministically holds the budget out of the
// way and the per-receiver count fixed, so the amount of work is as fixed as
// its selection, and asks for the same estimate twice.
func TestNormativeEstimateSamplesDeterministically(t *testing.T) {
	t.Parallel()

	receivers, scene := probeNormativeScene(t, 600, 4)
	opts := runestimate.Options{
		Budget:              time.Minute,
		MaxProbeReceivers:   4,
		MaxUnitsPerReceiver: 5,
		Workers:             4,
	}

	first, err := EstimateNormative(t.Context(), receivers, scene, opts)
	if err != nil {
		t.Fatal(err)
	}

	second, err := EstimateNormative(t.Context(), receivers, scene, opts)
	if err != nil {
		t.Fatal(err)
	}

	parts, err := prepareSpeedZoneParts(scene.Segments)
	if err != nil {
		t.Fatal(err)
	}

	kernel := newNormativeKernel(receivers, scene, parts)
	points := normativePoints(receivers)

	assertSamplesThePlan(t, points, kernel.Units, first, second, 4, 5)

	// The chain walks its receivers on one goroutine, whatever the caller
	// asked for.
	if first.Inputs.Workers != 1 || first.Inputs.Units != int64(12*len(receivers)) {
		t.Fatalf("inputs %+v, want 1 worker and 12 units per receiver", first.Inputs)
	}

	wantCounts := map[string]int{
		"track_segments": 2, "speed_zone_parts": 4, "line_source_walks": 12,
		"subsegments": first.Inputs.Counts["subsegments"], "walls": 16, "barrier_panels": 1,
	}
	if !reflect.DeepEqual(first.Inputs.Counts, wantCounts) || first.Inputs.Counts["subsegments"] < 100 {
		t.Fatalf("counts %v, want %v with the 600 m and 400 m lines' 10 m steps", first.Inputs.Counts, wantCounts)
	}

	if first.StandardID != StandardID || !first.Available {
		t.Fatalf("estimate %q available=%v", first.StandardID, first.Available)
	}
}

// TestPreviewEstimateSamplesDeterministically is the same for the preview
// chain.
func TestPreviewEstimateSamplesDeterministically(t *testing.T) {
	t.Parallel()

	receivers, sources := probePreviewScene(12, 4)
	cfg, pack := DefaultPropagationConfig(), BuiltinDataPack()
	opts := runestimate.Options{
		Budget:              time.Minute,
		MaxProbeReceivers:   4,
		MaxUnitsPerReceiver: 5,
		Workers:             4,
	}

	first, err := EstimatePreview(t.Context(), receivers, sources, cfg, pack, opts)
	if err != nil {
		t.Fatal(err)
	}

	second, err := EstimatePreview(t.Context(), receivers, sources, cfg, pack, opts)
	if err != nil {
		t.Fatal(err)
	}

	kernel := newPreviewKernel(receivers, sources, cfg, pack)
	points := previewPoints(receivers)

	assertSamplesThePlan(t, points, kernel.Units, first, second, 4, 5)

	if first.Inputs.Workers != 1 || first.Inputs.Units != int64(len(sources)*len(receivers)) {
		t.Fatalf("inputs %+v, want 1 worker and one unit per source and receiver", first.Inputs)
	}

	if first.Inputs.Counts["rail_sources"] != len(sources) || first.Inputs.Counts["subsegments"] < len(sources)*60 {
		t.Fatalf("counts %v", first.Inputs.Counts)
	}

	if first.StandardID != StandardID || !first.Available {
		t.Fatalf("estimate %q available=%v", first.StandardID, first.Available)
	}
}

// assertSamplesThePlan holds two estimates of one run to the deterministic
// sample plan: both timed exactly the units the plan names, measured every
// probe receiver, and scaled by the same inputs.
func assertSamplesThePlan(
	t *testing.T,
	points []geo.Point2D,
	units func(int) int,
	first, second runestimate.Estimate,
	limit, perReceiver int,
) {
	t.Helper()

	plan := runestimate.SamplePlan(points, units, limit, perReceiver)
	if again := runestimate.SamplePlan(points, units, limit, perReceiver); !reflect.DeepEqual(plan, again) {
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
}

// TestNormativeEstimateRefusesWhatTheRunRefuses keeps the estimate from
// answering a question the run would refuse, and holds it to the run's words.
// Each case breaks a different check: the first receiver and a later one, a
// wall, a barrier panel, a segment, a track feature the speed-zone split
// rejects, an emission, and the two empty inputs.
func TestNormativeEstimateRefusesWhatTheRunRefuses(t *testing.T) {
	t.Parallel()

	cases := map[string]func([]ReceiverInput, *NormativeScene) []ReceiverInput{
		"a later receiver": func(r []ReceiverInput, _ *NormativeScene) []ReceiverInput {
			r[3].ID = ""

			return r
		},
		"the first receiver": func(r []ReceiverInput, _ *NormativeScene) []ReceiverInput {
			r[0].HeightM = math.NaN()

			return r
		},
		"a wall": func(r []ReceiverInput, s *NormativeScene) []ReceiverInput {
			s.Walls[5].HeightM = -1

			return r
		},
		"a barrier": func(r []ReceiverInput, s *NormativeScene) []ReceiverInput {
			s.Barriers[0].TopHeightM = 0

			return r
		},
		"a track feature off the track": func(r []ReceiverInput, s *NormativeScene) []ReceiverInput {
			s.Segments[1].Features = append(s.Segments[1].Features, TrackFeature{Kind: TrackFeatureWeiche, Point: geo.Point2D{X: 100, Y: 80}})

			return r
		},
		"an unknown vehicle": func(r []ReceiverInput, s *NormativeScene) []ReceiverInput {
			s.Segments[0].Operations[2].FzComposition[0].Fz = 999

			return r
		},
		// Segment validation passes this; only the emission refuses it.
		"an emission": func(r []ReceiverInput, s *NormativeScene) []ReceiverInput {
			op := &s.Segments[0].Operations[2]
			op.FzComposition = append(op.FzComposition, FzCount{Fz: 21, Count: 1})

			return r
		},
		"no receivers": func([]ReceiverInput, *NormativeScene) []ReceiverInput {
			return nil
		},
		"no segments": func(r []ReceiverInput, s *NormativeScene) []ReceiverInput {
			s.Segments = nil

			return r
		},
	}

	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			receivers, scene := probeNormativeScene(t, 600, 3)
			receivers = breakIt(receivers, &scene)

			_, want := ComputeNormativeReceiverOutputsForScene(receivers, scene)

			_, err := EstimateNormative(t.Context(), receivers, scene, runestimate.Options{Budget: time.Second})
			if err == nil || want == nil || err.Error() != want.Error() {
				t.Fatalf("estimate refused with %v, the run with %v", err, want)
			}

			t.Logf("refused: %v", err)
		})
	}
}

// TestPreviewEstimateRefusesWhatTheRunRefuses is the same for the preview
// chain.
func TestPreviewEstimateRefusesWhatTheRunRefuses(t *testing.T) {
	t.Parallel()

	type scene struct {
		receivers []geo.PointReceiver
		sources   []RailSource
		cfg       PropagationConfig
	}

	cases := map[string]func(*scene){
		"a later receiver":        func(s *scene) { s.receivers[3].ID = "" },
		"the first receiver":      func(s *scene) { s.receivers[0].Point.X = math.Inf(1) },
		"the propagation config":  func(s *scene) { s.cfg.AirAbsorptionDBPerKM = -1 },
		"a source":                func(s *scene) { s.sources[2].AverageSpeedKPH = 0 },
		"no sources":              func(s *scene) { s.sources = nil },
		"no receivers":            func(s *scene) { s.receivers = nil },
		"a source and a receiver": func(s *scene) { s.sources[1].ID = ""; s.receivers[2].ID = "" },
	}

	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			receivers, sources := probePreviewScene(4, 3)
			s := scene{receivers: receivers, sources: sources, cfg: DefaultPropagationConfig()}
			breakIt(&s)

			_, want := ComputeReceiverOutputsWithDataPack(s.receivers, s.sources, s.cfg, BuiltinDataPack())

			_, err := EstimatePreview(t.Context(), s.receivers, s.sources, s.cfg, BuiltinDataPack(), runestimate.Options{Budget: time.Second})
			if err == nil || want == nil || err.Error() != want.Error() {
				t.Fatalf("estimate refused with %v, the run with %v", err, want)
			}
		})
	}
}

// TestNormativeEstimateIsWithinAFactorOfTheRun runs the real walk over the
// scene and holds the estimate to it, within a factor of three either way.
//
// Both sides are read on the clock the probe times units on, for the reason
// rls19road's TestEstimateIsWithinAFactorOfTheRun gives: the thread CPU clock
// where the platform has one, the wall clock otherwise.
func TestNormativeEstimateIsWithinAFactorOfTheRun(t *testing.T) {
	if testing.Short() {
		t.Skip("times a run")
	}

	receivers, scene := probeNormativeScene(t, 8000, 10)

	estimate, err := EstimateNormative(t.Context(), receivers, scene, runestimate.Options{Budget: 200 * time.Millisecond, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}

	assertWithinAFactorOfTheRun(t, estimate, func() error {
		_, err := ComputeNormativeReceiverOutputsForScene(receivers, scene)

		return err
	})
}

// TestPreviewEstimateIsWithinAFactorOfTheRun is the same for the preview
// chain.
func TestPreviewEstimateIsWithinAFactorOfTheRun(t *testing.T) {
	if testing.Short() {
		t.Skip("times a run")
	}

	receivers, sources := probePreviewScene(60, 12)
	cfg, pack := DefaultPropagationConfig(), BuiltinDataPack()

	estimate, err := EstimatePreview(t.Context(), receivers, sources, cfg, pack, runestimate.Options{Budget: 150 * time.Millisecond, Workers: 1})
	if err != nil {
		t.Fatal(err)
	}

	assertWithinAFactorOfTheRun(t, estimate, func() error {
		_, err := ComputeReceiverOutputsWithDataPack(receivers, sources, cfg, pack)

		return err
	})
}

// assertWithinAFactorOfTheRun times run on this goroutine, with the thread
// held so its CPU time is the run's, and holds the estimate to it within a
// factor of three either way.
func assertWithinAFactorOfTheRun(t *testing.T, estimate runestimate.Estimate, run func() error) {
	t.Helper()

	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	read := func() time.Duration {
		if cpu, ok := runestimate.ThreadCPUTime(); ok {
			return cpu
		}

		return time.Duration(time.Now().UnixNano())
	}

	startedAt := read()

	err := run()
	if err != nil {
		t.Fatal(err)
	}

	measured := (read() - startedAt).Seconds()

	// One worker, so the estimate's compute is its CPU figure; the run
	// prepares no scene of its own beyond what every receiver repeats.
	estimated := estimate.CPUSeconds.Estimate + estimate.Probe.PrepareSeconds

	if estimated < measured/3 || estimated > measured*3 {
		t.Fatalf("estimated %.3fs (cpu band %.3f–%.3f, %d units timed, clock %s), the run took %.3fs",
			estimated, estimate.CPUSeconds.Low, estimate.CPUSeconds.High, estimate.Probe.UnitsTimed, estimate.Probe.Clock, measured)
	}

	t.Logf("estimated %.3fs (cpu band %.3f–%.3f, %d units timed, clock %s), measured %.3fs",
		estimated, estimate.CPUSeconds.Low, estimate.CPUSeconds.High, estimate.Probe.UnitsTimed,
		estimate.Probe.Clock, measured)
}
