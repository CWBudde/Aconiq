package road

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/runestimate"
)

// probeScene is a small scene with everything the walk can visit: a meandering
// road and a straight one, a row of buildings (barriers and reflectors at
// once), an explicit screen and a Parkplatz.
func probeScene(mode SegmentLengthMode) ([]RoadSource, []Barrier, PropagationConfig) {
	meander := make([]geo.Point2D, 41)
	for i := range meander {
		t := float64(i) / 40
		meander[i] = geo.Point2D{X: t * 400, Y: 8 * math.Sin(2*math.Pi*t*2)}
	}

	road := func(id string, line []geo.Point2D) RoadSource {
		return RoadSource{
			ID:          id,
			Centerline:  line,
			SurfaceType: SurfaceSMA,
			LaneCount:   2,
			Speeds:      SpeedInput{PkwKPH: 50, Lkw1KPH: 50, Lkw2KPH: 50, KradKPH: 50},
			TrafficDay:  TrafficInput{PkwPerHour: 600, Lkw1PerHour: 20, Lkw2PerHour: 15, KradPerHour: 5},
			TrafficNight: TrafficInput{
				PkwPerHour: 100, Lkw1PerHour: 5, Lkw2PerHour: 4, KradPerHour: 1,
			},
		}
	}

	sources := []RoadSource{
		road("meander", meander),
		road("cross", []geo.Point2D{{X: 200, Y: -150}, {X: 200, Y: 150}}),
	}

	buildings := make([]Building, 0, 8)

	for i := range 8 {
		x := 20 + float64(i)*45
		buildings = append(buildings, Building{
			ID:        fmt.Sprintf("b-%d", i),
			Footprint: []geo.Point2D{{X: x, Y: 25}, {X: x + 15, Y: 25}, {X: x + 15, Y: 37}, {X: x, Y: 37}},
			HeightM:   12,
		})
	}

	barriers := []Barrier{{ID: "screen", Geometry: []geo.Point2D{{X: 60, Y: -20}, {X: 160, Y: -20}}, HeightM: 4}}

	cfg := DefaultPropagationConfig()
	cfg.SegmentLengthM = 5
	cfg.SegmentLengthMode = mode
	cfg.Buildings = buildings
	cfg.ParkingSources = []ParkingSource{{
		ID:                     "lot",
		Center:                 geo.Point2D{X: 300, Y: -60},
		AreaM2:                 2000,
		NumSpaces:              80,
		LotType:                ParkingLotPkw,
		MovementsPerSpaceDay:   MovementRate(0.5),
		MovementsPerSpaceNight: MovementRate(0.1),
	}}

	return sources, barriers, cfg
}

func probeReceivers(side int) []geo.PointReceiver {
	receivers := make([]geo.PointReceiver, 0, 25)

	for iy := range side {
		for ix := range side {
			receivers = append(receivers, geo.PointReceiver{
				ID:      fmt.Sprintf("r-%d-%d", ix, iy),
				Point:   geo.Point2D{X: 10 + float64(ix)*360/float64(side), Y: 45 + float64(iy)*160/float64(side)},
				HeightM: 4 + float64(ix%2),
			})
		}
	}

	return receivers
}

// TestProbeUnitsAreTheWalksPairs pins the probe's units to the walk's pairs in
// the strongest way available: summing every unit's contributions, in unit
// order, has to give the walk's level to the bit. That holds only if the
// probe visits the same (source, Teilstück) pairs and Parkplätze as the walk,
// each exactly once, in the walk's order, with the walk's per-receiver
// config — in particular only if it reads the distance-scaled ladder exactly
// as levelFor hands it to the walk.
func TestProbeUnitsAreTheWalksPairs(t *testing.T) {
	t.Parallel()

	for _, mode := range []SegmentLengthMode{SegmentLengthFixed, SegmentLengthDistanceScaled} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			sources, barriers, cfg := probeScene(mode)
			receivers := probeReceivers(5)

			scene, err := PrepareSceneFor(receivers, sources, barriers, cfg)
			if err != nil {
				t.Fatal(err)
			}

			kernel := newEstimateKernel(scene, receivers, cfg)
			walker := kernel.newWalker()

			for i, r := range receivers {
				receiverCfg := cfg
				receiverCfg.ReceiverHeightM = r.HeightM

				want, err := scene.ComputeReceiverLevels(r.Point, receiverCfg)
				if err != nil {
					t.Fatal(err)
				}

				walker.Begin(i)
				walker.sums = periodSums{}

				units := kernel.Units(i)
				for u := range units {
					walker.addUnit(i, u)
				}

				got := PeriodLevels{LrDay: walker.sums.day.Level(), LrNight: walker.sums.night.Level()}

				if got != want {
					t.Fatalf("receiver %s: %d units sum to %+v, the walk computes %+v", r.ID, units, got, want)
				}
			}
		})
	}
}

// TestProbeUnitsFollowTheLadder checks the count itself: in fixed mode every
// receiver walks every Teilstück of rung zero, and in distance-scaled mode a
// receiver far from the roads walks fewer.
func TestProbeUnitsFollowTheLadder(t *testing.T) {
	t.Parallel()

	receivers := probeReceivers(5)

	fixedSources, barriers, fixedCfg := probeScene(SegmentLengthFixed)

	fixedScene, err := PrepareSceneFor(receivers, fixedSources, barriers, fixedCfg)
	if err != nil {
		t.Fatal(err)
	}

	rungZero := 0
	for i := range fixedScene.sources {
		rungZero += len(fixedScene.sources[i].segments)
	}

	fixed := newEstimateKernel(fixedScene, receivers, fixedCfg)

	scaledSources, _, scaledCfg := probeScene(SegmentLengthDistanceScaled)

	scaledScene, err := PrepareSceneFor(receivers, scaledSources, barriers, scaledCfg)
	if err != nil {
		t.Fatal(err)
	}

	scaled := newEstimateKernel(scaledScene, receivers, scaledCfg)

	far := len(receivers) - 1

	for i := range receivers {
		if got := fixed.Units(i); got != rungZero+1 {
			t.Fatalf("fixed receiver %d: %d units, want %d Teilstücke + 1 Parkplatz", i, got, rungZero)
		}
	}

	if scaled.Units(far) >= fixed.Units(far) {
		t.Fatalf("far receiver walks %d units distance-scaled, %d fixed: the ladder is not being read", scaled.Units(far), fixed.Units(far))
	}
}

// TestEstimateSamplesDeterministically holds the budget out of the way and
// the per-receiver count fixed, so the amount of work is as fixed as its
// selection, and asks for the same estimate twice.
func TestEstimateSamplesDeterministically(t *testing.T) {
	t.Parallel()

	sources, barriers, cfg := probeScene(SegmentLengthDistanceScaled)
	receivers := probeReceivers(5)
	opts := runestimate.Options{
		Budget:              time.Minute,
		MaxProbeReceivers:   4,
		MaxUnitsPerReceiver: 24,
		Workers:             2,
	}

	first, err := Estimate(t.Context(), receivers, sources, barriers, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}

	second, err := Estimate(t.Context(), receivers, sources, barriers, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}

	scene, err := PrepareSceneFor(receivers, sources, barriers, cfg)
	if err != nil {
		t.Fatal(err)
	}

	kernel := newEstimateKernel(scene, receivers, cfg)
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

	wantCounts := map[string]int{
		"road_sources": 2, "teilstuecke": scene.estimateCounts(0, cfg)["teilstuecke"], "buildings": 8,
		"reflector_walls": len(scene.reflectors.walls), "barriers": 1, "parking_sources": 1,
	}
	if !reflect.DeepEqual(first.Inputs.Counts, wantCounts) {
		t.Fatalf("counts %v, want %v", first.Inputs.Counts, wantCounts)
	}

	// Distance-scaled already: there is no cheaper mode to offer.
	if len(first.Alts) != 0 {
		t.Fatalf("a distance-scaled estimate offers %+v", first.Alts)
	}

	fixedSources, _, fixedCfg := probeScene(SegmentLengthFixed)

	fixed, err := Estimate(t.Context(), receivers, fixedSources, barriers, fixedCfg, opts)
	if err != nil {
		t.Fatal(err)
	}

	// The alternative is a distance-scaled run's own unit count.
	if len(fixed.Alts) != 1 || fixed.Alts[0].Params["segment_length_mode"] != string(SegmentLengthDistanceScaled) ||
		fixed.Alts[0].Units != first.Inputs.Units {
		t.Fatalf("a fixed-mode estimate should cost the distance-scaled run (%d units), got %+v", first.Inputs.Units, fixed.Alts)
	}
}

// TestEstimateRefusesWhatTheRunRefuses keeps the estimate from answering a
// question the run would refuse.
func TestEstimateRefusesWhatTheRunRefuses(t *testing.T) {
	t.Parallel()

	sources, barriers, cfg := probeScene(SegmentLengthFixed)
	receivers := probeReceivers(5)
	receivers[3].ID = ""

	_, want := ComputeReceiverOutputs(receivers, sources, barriers, cfg)

	_, err := Estimate(t.Context(), receivers, sources, barriers, cfg, runestimate.Options{Budget: time.Second})
	if err == nil || want == nil || err.Error() != want.Error() {
		t.Fatalf("estimate refused with %v, the run with %v", err, want)
	}
}

// TestEstimateIsWithinAFactorOfTheRun times the real walk over the scene and
// holds the estimate to it. The bound is a factor of three either way rather
// than the band, because a test machine running the rest of the suite in
// parallel is exactly the noise the band's systematic margin does not claim
// to cover — what this pins is that the extrapolation is of the right order.
func TestEstimateIsWithinAFactorOfTheRun(t *testing.T) {
	if testing.Short() {
		t.Skip("times a run")
	}

	sources, barriers, cfg := probeScene(SegmentLengthFixed)
	cfg.SegmentLengthM = 0.5

	receivers := probeReceivers(12)

	estimate, err := Estimate(t.Context(), receivers, sources, barriers, cfg, runestimate.Options{
		Budget:  150 * time.Millisecond,
		Workers: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	startedAt := time.Now()

	_, err = ComputeReceiverOutputs(receivers, sources, barriers, cfg)
	if err != nil {
		t.Fatal(err)
	}

	measured := time.Since(startedAt).Seconds()
	estimated := estimate.WallSeconds.Estimate

	if estimated < measured/3 || estimated > measured*3 {
		t.Fatalf("estimated %.3fs (band %.3f–%.3f), the run took %.3fs", estimated,
			estimate.WallSeconds.Low, estimate.WallSeconds.High, measured)
	}

	t.Logf("estimated %.3fs (band %.3f–%.3f, %d units timed), measured %.3fs",
		estimated, estimate.WallSeconds.Low, estimate.WallSeconds.High, estimate.Probe.UnitsTimed, measured)
}
