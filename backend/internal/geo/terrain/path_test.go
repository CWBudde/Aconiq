package terrain_test

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo/terrain"
)

// shapedTerrain answers from a function of x and y, and refuses everything
// outside its bounds — the two behaviours a real DTM has that matter here.
type shapedTerrain struct {
	minX, minY, maxX, maxY float64
	elevation              func(x, y float64) float64
}

func (s shapedTerrain) ElevationAt(x, y float64) (float64, bool) {
	if x < s.minX || x > s.maxX || y < s.minY || y > s.maxY {
		return 0, false
	}

	return s.elevation(x, y), true
}

func (s shapedTerrain) Bounds() [4]float64 {
	return [4]float64{s.minX, s.minY, s.maxX, s.maxY}
}

func (s shapedTerrain) Info() terrain.Info {
	return terrain.Info{Bounds: s.Bounds(), PixelSize: [2]float64{1, 1}, GridSize: [2]int{1, 1}}
}

func flatTerrainAt(elevationM float64) shapedTerrain {
	return shapedTerrain{
		minX: -1000, minY: -1000, maxX: 1000, maxY: 1000,
		elevation: func(_, _ float64) float64 { return elevationM },
	}
}

// Flat ground rises nowhere above the chord joining its own ends, whatever
// elevation it sits at. This is the property that keeps a site's altitude out
// of the acoustics.
func TestMeanRiseAboveChordIsZeroOnFlatGround(t *testing.T) {
	t.Parallel()

	for _, elevationM := range []float64{0, 400, -12.5} {
		rise, ok := terrain.MeanRiseAboveChord(flatTerrainAt(elevationM), 0, 0, 0, 200, 25)
		if !ok {
			t.Fatalf("flat terrain at %.1f m: no answer", elevationM)
		}

		if math.Abs(rise) > 1e-12 {
			t.Fatalf("flat terrain at %.1f m rises %.6f m above its own chord", elevationM, rise)
		}
	}
}

// A uniform slope is its own chord: the ground climbs, but never above the
// straight line joining its two ends.
func TestMeanRiseAboveChordIsZeroOnAUniformSlope(t *testing.T) {
	t.Parallel()

	slope := shapedTerrain{
		minX: -1000, minY: -1000, maxX: 1000, maxY: 1000,
		elevation: func(_, y float64) float64 { return 100 + 0.1*y },
	}

	rise, ok := terrain.MeanRiseAboveChord(slope, 0, 0, 0, 200, 25)
	if !ok {
		t.Fatal("uniform slope: no answer")
	}

	if math.Abs(rise) > 1e-9 {
		t.Fatalf("uniform slope rises %.9f m above its own chord", rise)
	}
}

// A ridge between the two ends rises above the chord; a valley falls below it.
// The sign is what a mean-height correction reads as "the ground comes up to
// meet the path" or "the ground drops away from it".
func TestMeanRiseAboveChordSignsARidgeAndAValley(t *testing.T) {
	t.Parallel()

	// A 20 m ridge at the midpoint of a 200 m path, falling linearly to zero
	// at both ends: mean rise = 10 m.
	ridge := shapedTerrain{
		minX: -1000, minY: -1000, maxX: 1000, maxY: 1000,
		elevation: func(_, y float64) float64 { return 20 * (1 - math.Abs(y-100)/100) },
	}

	rise, ok := terrain.MeanRiseAboveChord(ridge, 0, 0, 0, 200, 25)
	if !ok {
		t.Fatal("ridge: no answer")
	}

	if math.Abs(rise-10) > 0.2 {
		t.Fatalf("ridge mean rise = %.4f m, want about 10", rise)
	}

	valley := shapedTerrain{
		minX: -1000, minY: -1000, maxX: 1000, maxY: 1000,
		elevation: func(x, y float64) float64 { return -ridge.elevation(x, y) },
	}

	rise, ok = terrain.MeanRiseAboveChord(valley, 0, 0, 0, 200, 25)
	if !ok {
		t.Fatal("valley: no answer")
	}

	if math.Abs(rise+10) > 0.2 {
		t.Fatalf("valley mean rise = %.4f m, want about -10", rise)
	}
}

// The rise is measured against the terrain's own two endpoints, so a DTM on a
// different datum contributes its shape and not its offset.
func TestMeanRiseAboveChordIgnoresTheTerrainDatum(t *testing.T) {
	t.Parallel()

	shape := func(offset float64) shapedTerrain {
		return shapedTerrain{
			minX: -1000, minY: -1000, maxX: 1000, maxY: 1000,
			elevation: func(_, y float64) float64 {
				return offset + 20*(1-math.Abs(y-100)/100)
			},
		}
	}

	low, ok := terrain.MeanRiseAboveChord(shape(0), 0, 0, 0, 200, 25)
	if !ok {
		t.Fatal("no answer at datum 0")
	}

	high, ok := terrain.MeanRiseAboveChord(shape(1500), 0, 0, 0, 200, 25)
	if !ok {
		t.Fatal("no answer at datum 1500")
	}

	if math.Abs(low-high) > 1e-9 {
		t.Fatalf("the same shape 1500 m up gave %.9f instead of %.9f", high, low)
	}
}

// Nothing to sample is not a rise of zero dressed up as an answer: the caller
// has to be able to fall back on what else it knows about the ground.
func TestMeanRiseAboveChordRefusesWhatItCannotSample(t *testing.T) {
	t.Parallel()

	flat := flatTerrainAt(30)

	cases := []struct {
		name                  string
		model                 terrain.Model
		x0, y0, x1, y1, stepM float64
	}{
		{"no model", nil, 0, 0, 0, 200, 25},
		{"degenerate path", flat, 5, 5, 5, 5, 25},
		{"non-finite path", flat, 0, 0, math.NaN(), 200, 25},
		{"non-positive step", flat, 0, 0, 0, 200, 0},
		{"start outside the terrain", flat, -5000, 0, 0, 200, 25},
		{"end outside the terrain", flat, 0, 0, 0, 5000, 25},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, ok := terrain.MeanRiseAboveChord(tc.model, tc.x0, tc.y0, tc.x1, tc.y1, tc.stepM)
			if ok {
				t.Fatal("expected no answer")
			}
		})
	}
}

// A hole in the middle of an otherwise covered path is a gap in the sampling,
// not a sudden drop to sea level.
func TestMeanRiseAboveChordTreatsAnInteriorMissAsNoRise(t *testing.T) {
	t.Parallel()

	// Covered at both ends, with a square hole around the midpoint.
	holed := shapedTerrain{
		minX: -1000, minY: -1000, maxX: 1000, maxY: 1000,
		elevation: func(_, _ float64) float64 { return 300 },
	}
	gapped := gappedTerrain{inner: holed, gapFrom: 60, gapTo: 140}

	rise, ok := terrain.MeanRiseAboveChord(gapped, 0, 0, 0, 200, 25)
	if !ok {
		t.Fatal("gapped terrain: no answer")
	}

	if math.Abs(rise) > 1e-12 {
		t.Fatalf("the gap was read as ground at zero: rise = %.4f m", rise)
	}
}

// gappedTerrain refuses a band of y values in the middle of the path.
type gappedTerrain struct {
	inner          shapedTerrain
	gapFrom, gapTo float64
}

func (g gappedTerrain) ElevationAt(x, y float64) (float64, bool) {
	if y > g.gapFrom && y < g.gapTo {
		return 0, false
	}

	return g.inner.ElevationAt(x, y)
}

func (g gappedTerrain) Bounds() [4]float64 { return g.inner.Bounds() }

func (g gappedTerrain) Info() terrain.Info { return g.inner.Info() }
