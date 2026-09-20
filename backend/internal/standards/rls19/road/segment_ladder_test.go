package road_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	road "github.com/aconiq/backend/internal/standards/rls19/road"
)

// ladderSource is one straight way of the given length, laid along y = 0 from
// the origin. An OSM extract arrives as one source per way and they average
// some sixty metres, which is the length the ladder's granularity argument
// rests on, so the default here is that.
func ladderSource(id string, lengthM float64) road.RoadSource {
	return road.RoadSource{
		ID:          id,
		Centerline:  []geo.Point2D{{X: 0, Y: 0}, {X: lengthM, Y: 0}},
		SurfaceType: road.SurfaceSMA,
		Speeds: road.SpeedInput{
			PkwKPH: 50, Lkw1KPH: 50, Lkw2KPH: 50, KradKPH: 50,
		},
		TrafficDay: road.TrafficInput{
			PkwPerHour: 800, Lkw1PerHour: 40, Lkw2PerHour: 30, KradPerHour: 10,
		},
		TrafficNight: road.TrafficInput{
			PkwPerHour: 200, Lkw1PerHour: 10, Lkw2PerHour: 8, KradPerHour: 2,
		},
	}
}

func ladderConfig(mode road.SegmentLengthMode) road.PropagationConfig {
	cfg := road.DefaultPropagationConfig()
	cfg.SegmentLengthM = 1
	cfg.SegmentLengthMode = mode

	return cfg
}

func levelsAt(
	t *testing.T,
	sources []road.RoadSource,
	receivers []geo.PointReceiver,
	cfg road.PropagationConfig,
) []road.ReceiverOutput {
	t.Helper()

	outputs, err := road.ComputeReceiverOutputs(receivers, sources, nil, cfg)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}

	return outputs
}

// The zero value is fixed mode, byte for byte.
//
// This is the guarantee that let the parameter be added without re-cutting a
// single golden: every caller that predates it — the acceptance runner, the
// WASM kernel's request decoder, every test that builds a config literal —
// leaves the field empty and must be unable to tell that it exists.
func TestUnsetModeIsByteIdenticalToFixed(t *testing.T) {
	t.Parallel()

	sources := []road.RoadSource{ladderSource("w", 60)}
	receivers := []geo.PointReceiver{
		{ID: "near", Point: geo.Point2D{X: 30, Y: 8}, HeightM: 4},
		{ID: "far", Point: geo.Point2D{X: 30, Y: 400}, HeightM: 4},
	}

	unset := levelsAt(t, sources, receivers, ladderConfig(""))
	fixed := levelsAt(t, sources, receivers, ladderConfig(road.SegmentLengthFixed))

	unsetJSON, err := json.Marshal(unset)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	fixedJSON, err := json.Marshal(fixed)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if string(unsetJSON) != string(fixedJSON) {
		t.Fatalf("the zero value is not fixed mode:\n unset = %s\n fixed = %s", unsetJSON, fixedJSON)
	}
}

// An unknown mode is refused rather than quietly read as one of the two.
func TestValidateRefusesAnUnknownMode(t *testing.T) {
	t.Parallel()

	cfg := ladderConfig("adaptive")

	err := cfg.Validate()
	if err == nil {
		t.Fatal("an unknown segment_length_mode was accepted")
	}

	if !strings.Contains(err.Error(), "segment_length_mode") {
		t.Fatalf("the refusal does not name the parameter: %v", err)
	}
}

// A receiver close enough that the Anmerkung's bound is tighter than the
// length the user asked for reads the length the user asked for.
//
// The mode is a licence to coarsen up to l <= s/2, never an instruction to
// split finer than segment_length_m. A receiver 1 m from the road would admit
// a 0.5 m Teilstück; it must still get 1 m.
func TestDistanceScaledNeverSplitsFinerThanAsked(t *testing.T) {
	t.Parallel()

	sources := []road.RoadSource{ladderSource("w", 60)}
	receivers := []geo.PointReceiver{{ID: "hugging", Point: geo.Point2D{X: 30, Y: 1}, HeightM: 4}}

	fixed := levelsAt(t, sources, receivers, ladderConfig(road.SegmentLengthFixed))
	scaled := levelsAt(t, sources, receivers, ladderConfig(road.SegmentLengthDistanceScaled))

	if math.Abs(fixed[0].Indicators.LrDay-scaled[0].Indicators.LrDay) > 0 {
		t.Fatalf("a receiver 1 m from the road moved: fixed %.12f, scaled %.12f",
			fixed[0].Indicators.LrDay, scaled[0].Indicators.LrDay)
	}
}

// The convergence evidence.
//
// distance_scaled is a declared model parameter, not an optimisation, because
// it changes the level. What it may not do is change it by an amount that
// matters at the 0.1 dB the results are reported to — and the Anmerkung to
// RLS-19 Nr. 3.2 publishes l_i <= s_i/2 as exactly the bound under which it
// does not.
//
// This walks a receiver away from a 60 m way and records what the coarsening
// costs at each distance. Measured on this fixture the worst deviation is
// +0.066 dB, at 150 m — where the ladder has just taken the rung that holds
// the whole way in two Teilstücke — and it falls away on both sides of that:
// nearer, because the bound stops admitting coarser rungs; further, because
// the way subtends less and less angle.
//
// Two things about the sign are worth stating, because only one of them is a
// guarantee. Every deviation on this fixture is *positive*: coarsening
// concentrates the segment's power at fewer midpoints, and the nearest of
// those sits closer to the receiver than the segment's mean distance, so the
// level comes out slightly high. Over-predicting is the direction a deviation
// has to err in to be defensible in a 16. BImSchV context, and it is the
// direction every already-declared RLS-19 deviation errs in. But it is an
// observation about this geometry, not a theorem, which is why the assertion
// below is on the magnitude and the sign is only recorded.
//
// The band is 0.1 dB, the resolution results are reported to. That is the
// honest threshold rather than a flattering one: at 0.066 dB the mode can
// still move a receiver across a rounding boundary and change the reported
// value by one step, and the conformance declaration says so.
func TestDistanceScaledStaysInsideTheAnmerkungBound(t *testing.T) {
	t.Parallel()

	sources := []road.RoadSource{ladderSource("w", 60)}

	// Below the road's own half-length the rule admits no coarsening worth
	// having, and beyond a few hundred metres the whole way is one Teilstück
	// and the error stops growing. The interesting range is in between.
	distances := []float64{5, 10, 25, 50, 75, 100, 150, 200, 300, 500, 800}

	receivers := make([]geo.PointReceiver, 0, len(distances))
	for _, d := range distances {
		receivers = append(receivers, geo.PointReceiver{
			ID:      fmt.Sprintf("d%.0f", d),
			Point:   geo.Point2D{X: 30, Y: d},
			HeightM: 4,
		})
	}

	fixed := levelsAt(t, sources, receivers, ladderConfig(road.SegmentLengthFixed))
	scaled := levelsAt(t, sources, receivers, ladderConfig(road.SegmentLengthDistanceScaled))

	const bandDB = 0.1

	worst, mostNegative := 0.0, 0.0

	for i := range fixed {
		deltaDay := scaled[i].Indicators.LrDay - fixed[i].Indicators.LrDay
		deltaNight := scaled[i].Indicators.LrNight - fixed[i].Indicators.LrNight

		t.Logf("%5.0f m  day %+.6f dB  night %+.6f dB", distances[i], deltaDay, deltaNight)

		worst = math.Max(worst, math.Max(math.Abs(deltaDay), math.Abs(deltaNight)))
		mostNegative = math.Min(mostNegative, math.Min(deltaDay, deltaNight))
	}

	t.Logf("worst |delta| %.6f dB, most negative delta %.6f dB", worst, mostNegative)

	if worst > bandDB {
		t.Fatalf("worst deviation %.6f dB exceeds the %.2f dB band the l<=s/2 rule is supposed to buy",
			worst, bandDB)
	}
}

// The ladder must not make the answer depend on how the receivers were split
// across workers.
//
// Choosing a rung is a per-receiver decision, which is exactly the kind of
// thing that could have made a chunk's result depend on its neighbours. It
// does not, because the rungs are built once in PrepareScene from the model
// alone and a receiver only reads one — but that is a claim, and this is the
// check docs/policies/determinism.md asks for.
func TestDistanceScaledIsInvariantToWorkerCount(t *testing.T) {
	t.Parallel()

	sources := []road.RoadSource{
		ladderSource("near", 60),
		ladderSource("far", 300),
	}
	sources[1].Centerline = []geo.Point2D{{X: -100, Y: 420}, {X: 200, Y: 460}}

	// Prime, so no worker count in the list divides it.
	const receiverCount = 97

	receivers := make([]geo.PointReceiver, 0, receiverCount)
	for i := range receiverCount {
		receivers = append(receivers, geo.PointReceiver{
			ID:      fmt.Sprintf("r%02d", i),
			Point:   geo.Point2D{X: float64(i) * 3, Y: 10 + float64(i)*4},
			HeightM: 4,
		})
	}

	cfg := ladderConfig(road.SegmentLengthDistanceScaled)

	hashes := make(map[string][]int)

	for _, workers := range []int{1, 2, 3, 4, 8, 17} {
		outputs, err := road.ComputeReceiverOutputsParallel(
			t.Context(), receivers, sources, nil, cfg, workers,
		)
		if err != nil {
			t.Fatalf("workers=%d: %v", workers, err)
		}

		encoded, err := json.Marshal(outputs)
		if err != nil {
			t.Fatalf("workers=%d: marshal: %v", workers, err)
		}

		sum := sha256.Sum256(encoded)
		hashes[hex.EncodeToString(sum[:])] = append(hashes[hex.EncodeToString(sum[:])], workers)
	}

	if len(hashes) != 1 {
		t.Fatalf("worker counts disagreed about the output: %v", hashes)
	}
}

// The mode has to actually do something, or the tests above pass vacuously.
//
// A level that does not move between the two modes at a distance where the
// ladder should have coarsened would mean the ladder is never selected — the
// failure a convergence test cannot see, because "identical" is the best
// possible convergence.
func TestDistanceScaledActuallyCoarsens(t *testing.T) {
	t.Parallel()

	sources := []road.RoadSource{ladderSource("w", 60)}
	receivers := []geo.PointReceiver{{ID: "far", Point: geo.Point2D{X: 30, Y: 300}, HeightM: 4}}

	fixed := levelsAt(t, sources, receivers, ladderConfig(road.SegmentLengthFixed))
	scaled := levelsAt(t, sources, receivers, ladderConfig(road.SegmentLengthDistanceScaled))

	if fixed[0].Indicators.LrDay == scaled[0].Indicators.LrDay {
		t.Fatal("300 m from a 60 m way the two modes agreed exactly, so no rung was taken")
	}
}
