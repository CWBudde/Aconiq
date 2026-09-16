package cli

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo/terrain"
)

// degreeGridTerrain stands in for a DTM imported into a geographic project:
// its grid covers a patch of lon/lat and it answers nothing outside it.
type degreeGridTerrain struct {
	minX, minY, maxX, maxY float64
	elevation              float64
	lastX, lastY           float64
}

func (g *degreeGridTerrain) ElevationAt(x, y float64) (float64, bool) {
	g.lastX, g.lastY = x, y

	if x < g.minX || x > g.maxX || y < g.minY || y > g.maxY {
		return 0, false
	}

	return g.elevation, true
}

func (g *degreeGridTerrain) Bounds() [4]float64 {
	return [4]float64{g.minX, g.minY, g.maxX, g.maxY}
}

func (g *degreeGridTerrain) Info() terrain.Info {
	return terrain.Info{Bounds: g.Bounds(), PixelSize: [2]float64{0.001, 0.001}, GridSize: [2]int{10, 10}}
}

func hamburgDegreeTerrain() *degreeGridTerrain {
	return &degreeGridTerrain{minX: 9.99, minY: 53.54, maxX: 10.01, maxY: 53.56, elevation: 31.5}
}

// The defect: the model is projected into metres and the terrain is not, so
// every ElevationAt arrives in UTM against a grid in degrees, falls outside
// its bounds, and terrainElevationAt turns the miss into an elevation of 0
// without a word. Receiver heights and the propagation path are then wrong,
// and nothing says so.
func TestTerrainInComputeCRSTransformsTheQuery(t *testing.T) {
	t.Parallel()

	inner := hamburgDegreeTerrain()

	wrapped, err := newTerrainInComputeCRS(inner, computeProjection{
		ProjectCRS: "EPSG:4326", ComputeCRS: "EPSG:25832", Applied: true,
	})
	if err != nil {
		t.Fatalf("newTerrainInComputeCRS: %v", err)
	}

	// (10.0, 53.55) in EPSG:25832, which is inside the degree grid once the
	// query is transformed back and nowhere near it if it is not.
	elevation, ok := wrapped.ElevationAt(566252.0, 5934021.0)
	if !ok {
		t.Fatalf("the query fell outside the terrain; it was passed through in metres (grid saw %.6f, %.6f)",
			inner.lastX, inner.lastY)
	}

	if elevation != 31.5 {
		t.Fatalf("elevation = %v, want 31.5", elevation)
	}

	if math.Abs(inner.lastX-10.0) > 0.01 || math.Abs(inner.lastY-53.55) > 0.01 {
		t.Fatalf("the grid was queried at (%.6f, %.6f), want roughly (10.0, 53.55)", inner.lastX, inner.lastY)
	}
}

// A point genuinely outside the terrain is still a miss, not an elevation of
// zero dressed up as data.
func TestTerrainInComputeCRSStillReportsAMiss(t *testing.T) {
	t.Parallel()

	wrapped, err := newTerrainInComputeCRS(hamburgDegreeTerrain(), computeProjection{
		ProjectCRS: "EPSG:4326", ComputeCRS: "EPSG:25832", Applied: true,
	})
	if err != nil {
		t.Fatalf("newTerrainInComputeCRS: %v", err)
	}

	// Roughly 200 km north of the grid.
	_, ok := wrapped.ElevationAt(566252.0, 6134021.0)
	if ok {
		t.Fatal("a point outside the terrain was reported as a hit")
	}
}

// The ordinary projected project must keep querying its grid directly; a
// wrapper there would be a transform between a CRS and itself on every lookup.
func TestTerrainInComputeCRSIsNotAppliedWhenTheModelDidNotMove(t *testing.T) {
	t.Parallel()

	inner := hamburgDegreeTerrain()

	wrapped, err := newTerrainInComputeCRS(inner, computeProjection{
		ProjectCRS: "EPSG:25832", ComputeCRS: "EPSG:25832", Applied: false,
	})
	if err != nil {
		t.Fatalf("newTerrainInComputeCRS: %v", err)
	}

	if wrapped != terrain.Model(inner) {
		t.Fatalf("the terrain was wrapped although the model did not move: %T", wrapped)
	}
}

func TestTerrainInComputeCRSPassesNilThrough(t *testing.T) {
	t.Parallel()

	wrapped, err := newTerrainInComputeCRS(nil, computeProjection{
		ProjectCRS: "EPSG:4326", ComputeCRS: "EPSG:25832", Applied: true,
	})
	if err != nil {
		t.Fatalf("newTerrainInComputeCRS: %v", err)
	}

	if wrapped != nil {
		t.Fatalf("a project with no terrain gained one: %T", wrapped)
	}
}

// Bounds and Info describe the stored artifact and stay in its own CRS —
// projecting a rectangle does not produce a rectangle, so any [4]float64
// answer would be wrong somewhere.
func TestTerrainInComputeCRSReportsTheArtifactsOwnExtent(t *testing.T) {
	t.Parallel()

	inner := hamburgDegreeTerrain()

	wrapped, err := newTerrainInComputeCRS(inner, computeProjection{
		ProjectCRS: "EPSG:4326", ComputeCRS: "EPSG:25832", Applied: true,
	})
	if err != nil {
		t.Fatalf("newTerrainInComputeCRS: %v", err)
	}

	if wrapped.Bounds() != inner.Bounds() {
		t.Fatalf("Bounds = %v, want the artifact's own %v", wrapped.Bounds(), inner.Bounds())
	}

	if wrapped.Info().GridSize != inner.Info().GridSize {
		t.Fatalf("Info lost the grid size: %v", wrapped.Info())
	}
}
