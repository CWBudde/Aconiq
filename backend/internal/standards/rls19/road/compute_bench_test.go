package road

import (
	"fmt"
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// The benchmark scene is deliberately the shape a real run has rather than the
// shape a unit test has: a 2 km road carried by ~100 centreline vertices, split
// at the default 1 m Teilstück length, against receiver grids up to the 10 000
// points a city-block raster produces. The per-receiver cost of re-deriving the
// scene only shows up at that ratio — with two vertices and ten receivers the
// segmentation disappears into the propagation loop.
const (
	benchRoadLengthM   = 2000.0
	benchRoadVertices  = 101 // 100 edges over benchRoadLengthM
	benchRoadAmplitude = 15.0
	benchBuildingCount = 200
)

// benchRoadSource builds a gently meandering 2 km road. The meander matters:
// interpolateAlongPolyline walks the polyline from vertex 0 for every segment,
// so a straight two-vertex line would hide exactly the cost this benchmark
// exists to measure.
func benchRoadSource() RoadSource {
	centerline := make([]geo.Point2D, benchRoadVertices)
	for i := range centerline {
		t := float64(i) / float64(benchRoadVertices-1)
		centerline[i] = geo.Point2D{
			X: t * benchRoadLengthM,
			Y: benchRoadAmplitude * math.Sin(2*math.Pi*t*3),
		}
	}

	return RoadSource{
		ID:          "bench-road",
		Centerline:  centerline,
		SurfaceType: SurfaceSMA,
		LaneCount:   4,
		Speeds: SpeedInput{
			PkwKPH: 100, Lkw1KPH: 80, Lkw2KPH: 70, KradKPH: 100,
		},
		TrafficDay: TrafficInput{
			PkwPerHour: 1200, Lkw1PerHour: 60, Lkw2PerHour: 80, KradPerHour: 15,
		},
		TrafficNight: TrafficInput{
			PkwPerHour: 300, Lkw1PerHour: 15, Lkw2PerHour: 25, KradPerHour: 3,
		},
	}
}

// benchReceivers lays out a square grid of count receivers on one side of the
// road, the way an auto-generated calculation grid arrives.
func benchReceivers(count int) []geo.PointReceiver {
	side := int(math.Round(math.Sqrt(float64(count))))
	if side*side != count {
		panic(fmt.Sprintf("benchReceivers: %d is not a square number", count))
	}

	receivers := make([]geo.PointReceiver, 0, count)

	for iy := range side {
		for ix := range side {
			receivers = append(receivers, geo.PointReceiver{
				ID:      fmt.Sprintf("r-%d-%d", ix, iy),
				Point:   geo.Point2D{X: float64(ix) * (benchRoadLengthM / float64(side)), Y: 60 + float64(iy)*5},
				HeightM: 4,
			})
		}
	}

	return receivers
}

// benchBuildings places a row of small footprints between the road and the
// receiver grid, so both the barrier set and the reflector set are populated.
func benchBuildings(count int) []Building {
	buildings := make([]Building, 0, count)

	for i := range count {
		x := float64(i) * (benchRoadLengthM / float64(count))
		buildings = append(buildings, Building{
			ID: fmt.Sprintf("b-%d", i),
			Footprint: []geo.Point2D{
				{X: x, Y: 40},
				{X: x + 6, Y: 40},
				{X: x + 6, Y: 50},
				{X: x, Y: 50},
			},
			HeightM: 9,
		})
	}

	return buildings
}

func benchConfig(segmentLengthM float64, buildings []Building) PropagationConfig {
	cfg := DefaultPropagationConfig()
	cfg.SegmentLengthM = segmentLengthM
	cfg.Buildings = buildings

	return cfg
}

func BenchmarkComputeReceiverOutputs(b *testing.B) {
	sources := []RoadSource{benchRoadSource()}

	for _, count := range []int{100, 2500, 10000} {
		b.Run(fmt.Sprintf("receivers=%d", count), func(b *testing.B) {
			receivers := benchReceivers(count)
			cfg := benchConfig(1, nil)

			b.ReportAllocs()

			for b.Loop() {
				_, err := ComputeReceiverOutputs(receivers, sources, nil, cfg)
				if err != nil {
					b.Fatalf("compute failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkComputeReceiverOutputsBuildings populates both derived sets — a
// Building is a barrier and a reflector at once — so the per-receiver cost of
// rebuilding them is on the clock.
//
// It runs at a 500 m Teilstück length, not the 1 m the grid benchmark uses,
// because one Teilstück against one receiver still costs single-digit
// milliseconds with 200 footprints in the scene, so a 1 m split would put a
// single iteration of this benchmark far past any useful runtime. The coarse
// split still exercises the whole barrier/reflector path; what it cannot do is
// stand in for a real 200-building run, which this engine cannot compute at
// grid scale.
//
// What holds that cost is the *number* of mirrored paths: ~307 valid
// Spiegelschallquellen per (Teilstück, receiver) pair, each of which RLS-19
// Nr. 3.5 treats as a source in its own right and therefore gives its own
// diffraction search. No exact prune reduces that count, because the model
// says those paths are there, and PLAN.md records why neither an occlusion
// test nor a cutoff is admissible. The search that finds them now runs from
// the receiver's side (reflections_receiver.go) and the diffraction search
// reads the receiver's barrier fan (shielding_fan.go); what is left per path
// is the crossing test against the barriers the ray actually passes.
func BenchmarkComputeReceiverOutputsBuildings(b *testing.B) {
	sources := []RoadSource{benchRoadSource()}
	buildings := benchBuildings(benchBuildingCount)

	b.Run("receivers=100", func(b *testing.B) {
		receivers := benchReceivers(100)
		cfg := benchConfig(500, buildings)

		b.ReportAllocs()

		for b.Loop() {
			_, err := ComputeReceiverOutputs(receivers, sources, nil, cfg)
			if err != nil {
				b.Fatalf("compute failed: %v", err)
			}
		}
	})
}

// BenchmarkComputeReceiverOutputsParallel is the same open-field grid as
// BenchmarkComputeReceiverOutputs, over a pool.
//
// workers=1 is not a separate implementation: ComputeReceiverOutputsParallel
// delegates below two workers, so that row is the sequential walk and the
// others are measured against it. The scaling is sub-linear and expected to
// be: the walk is memory-bound on the scene's arrays long before it is
// compute-bound, and hyperthreads add little.
func BenchmarkComputeReceiverOutputsParallel(b *testing.B) {
	sources := []RoadSource{benchRoadSource()}
	cfg := benchConfig(1, nil)

	for _, count := range []int{2500, 10000} {
		receivers := benchReceivers(count)

		for _, workers := range []int{1, 2, 4, 8} {
			b.Run(fmt.Sprintf("receivers=%d/workers=%d", count, workers), func(b *testing.B) {
				b.ReportAllocs()

				for b.Loop() {
					_, err := ComputeReceiverOutputsParallel(
						b.Context(), receivers, sources, nil, cfg, workers,
					)
					if err != nil {
						b.Fatalf("compute failed: %v", err)
					}
				}
			})
		}
	}
}

// BenchmarkComputeReceiverOutputsCity is the building-dense case: the synthetic
// city of city_scene_test.go, ~800 perimeter-block buildings with ~8 000 facade
// segments and ~35 km of road, which is the shape of the OSM extracts where
// the reflection search and the mirrored paths' shielding are nearly all of
// the cost.
//
// size=small is the same generator at three by three blocks, small enough to
// run every Teilstück at a 4 m split against four receivers. size=full is the
// whole city against one receiver of the patch, at the 1 m Teilstück length a
// real run uses, in distance-scaled mode so that a single iteration stays in
// the tens of seconds rather than the hours.
func BenchmarkComputeReceiverOutputsCity(b *testing.B) {
	cases := []struct {
		name           string
		spec           citySpec
		receivers      int
		segmentLengthM float64
		mode           SegmentLengthMode
	}{
		{"size=small", smallCity(), 4, 4, SegmentLengthFixed},
		{"size=full", fullCity(), 1, 1, SegmentLengthDistanceScaled},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			city := tc.spec.build()
			receivers := city.receivers[:tc.receivers]
			cfg := city.config(tc.segmentLengthM, tc.mode)

			b.ReportAllocs()

			for b.Loop() {
				_, err := ComputeReceiverOutputs(receivers, city.sources, city.barriers, cfg)
				if err != nil {
					b.Fatalf("compute failed: %v", err)
				}
			}
		})
	}
}
