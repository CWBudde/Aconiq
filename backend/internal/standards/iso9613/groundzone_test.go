package iso9613

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// zone builds a rectangular ground zone spanning [x0,x1] along the x axis and
// reaching far enough either side of the x axis to cover any path drawn on it.
func zone(id string, x0, x1, factor float64) GroundZone {
	return GroundZone{
		ID: id,
		Polygon: [][]geo.Point2D{{
			{X: x0, Y: -100}, {X: x1, Y: -100}, {X: x1, Y: 100}, {X: x0, Y: 100}, {X: x0, Y: -100},
		}},
		GroundFactor: factor,
	}
}

func TestResolveRegionFactorsWithoutZonesReproducesTheGlobalFactor(t *testing.T) {
	t.Parallel()

	gs, gr, gm := ResolveRegionFactors(
		geo.Point2D{X: 0, Y: 0}, geo.Point2D{X: 500, Y: 0}, 4, 4, nil, 0.5,
	)

	if gs != 0.5 || gr != 0.5 || gm != 0.5 {
		t.Fatalf("got gs=%v gr=%v gm=%v, want 0.5 for all three", gs, gr, gm)
	}
}

func TestResolveRegionFactors(t *testing.T) {
	t.Parallel()

	// h_s = h_r = 2 m, so each end region reaches 30·2 = 60 m and the middle
	// region is the 380 m between them.
	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 500, Y: 0}

	tests := []struct {
		name     string
		zones    []GroundZone
		fallback float64
		wantGs   float64
		wantGr   float64
		wantGm   float64
	}{
		{
			name:     "a zone over the source region moves gs alone",
			zones:    []GroundZone{zone("z", -10, 60, 1)},
			fallback: 0,
			wantGs:   1,
			wantGr:   0,
			wantGm:   0,
		},
		{
			name:     "a zone over the receiver region moves gr alone",
			zones:    []GroundZone{zone("z", 440, 510, 1)},
			fallback: 0,
			wantGs:   0,
			wantGr:   1,
			wantGm:   0,
		},
		{
			name:     "a zone over half the middle region gives a length-weighted gm",
			zones:    []GroundZone{zone("z", 60, 250, 1)},
			fallback: 0,
			wantGs:   0,
			wantGr:   0,
			wantGm:   0.5,
		},
		{
			name:     "ground no zone covers takes the fallback",
			zones:    []GroundZone{zone("z", 60, 250, 1)},
			fallback: 0.2,
			wantGs:   0.2,
			wantGr:   0.2,
			// Half the middle region at G=1, half at the 0.2 fallback.
			wantGm: 0.6,
		},
		{
			name: "the first zone wins where two overlap",
			zones: []GroundZone{
				zone("hard", -10, 60, 0),
				zone("porous", -10, 60, 1),
			},
			fallback: 0.5,
			wantGs:   0,
			wantGr:   0.5,
			wantGm:   0.5,
		},
		{
			name:     "a zone covering everything answers every region",
			zones:    []GroundZone{zone("z", -10, 510, 1)},
			fallback: 0,
			wantGs:   1,
			wantGr:   1,
			wantGm:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gs, gr, gm := ResolveRegionFactors(source, receiver, 2, 2, tc.zones, tc.fallback)

			if math.Abs(gs-tc.wantGs) > 1e-9 || math.Abs(gr-tc.wantGr) > 1e-9 || math.Abs(gm-tc.wantGm) > 1e-9 {
				t.Fatalf("got gs=%v gr=%v gm=%v, want gs=%v gr=%v gm=%v",
					gs, gr, gm, tc.wantGs, tc.wantGr, tc.wantGm)
			}
		})
	}
}

// TestResolveRegionFactorsWithoutAMiddleRegion pins the pairing with
// middleRegionQ: when the end regions meet, q is zero and A_m vanishes, so gm
// carries the fallback rather than a number that looks computed.
func TestResolveRegionFactorsWithoutAMiddleRegion(t *testing.T) {
	t.Parallel()

	// d_p = 100 m with h_s = h_r = 2 m: 30·(2+2) = 120 m ≥ d_p.
	source := geo.Point2D{X: 0, Y: 0}
	receiver := geo.Point2D{X: 100, Y: 0}

	if q := middleRegionQ(2, 2, 100); q != 0 {
		t.Fatalf("this case is meant to have no middle region, but q = %v", q)
	}

	_, _, gm := ResolveRegionFactors(source, receiver, 2, 2, []GroundZone{zone("z", -10, 110, 1)}, 0.3)
	if gm != 0.3 {
		t.Fatalf("gm = %v, want the 0.3 fallback where there is no middle region", gm)
	}
}

// TestBandAttenuationHonoursGroundZones is the end of the wire: a zone under
// the path has to change A_gr, and an empty scene has to leave the numbers
// exactly where the single global factor put them.
func TestBandAttenuationHonoursGroundZones(t *testing.T) {
	t.Parallel()

	receiver := geo.PointReceiver{ID: "r1", Point: geo.Point2D{X: 500, Y: 0}, HeightM: 2}
	source := PointSource{ID: "s1", Point: geo.Point2D{X: 0, Y: 0}, SourceHeightM: 2, SoundPowerLevelDB: 100}

	cfg := DefaultPropagationConfig()
	cfg.GroundFactor = 0

	hard, _ := BandAttenuation(receiver, source, cfg)

	zoned := cfg
	zoned.GroundZones = []GroundZone{zone("porous", -10, 510, 1)}

	porous, _ := BandAttenuation(receiver, source, zoned)

	if porous == hard {
		t.Fatal("a porous zone over the whole path left every band unchanged")
	}

	// Same scene, expressed the old way: a global factor of 1 and no zones has
	// to give exactly what a zone of 1 covering everything gives.
	global := cfg
	global.GroundFactor = 1

	wanted, _ := BandAttenuation(receiver, source, global)

	for i := range NumBands {
		if math.Abs(porous[i]-wanted[i]) > 1e-12 {
			t.Fatalf("band %d: zoned %v, global %v", i, porous[i], wanted[i])
		}
	}
}
