package road

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/terrain"
)

// levelsAtDatum computes one scene — one road, one receiver, flat ground —
// with the whole scene lifted to an absolute elevation.
func levelsAtDatum(t *testing.T, datumZ float64) PeriodLevels {
	t.Helper()

	source := sampleSource()
	source.ElevationM = datumZ

	cfg := DefaultPropagationConfig()
	cfg.ReceiverHeightM = 4
	cfg.ReceiverTerrainZ = datumZ

	levels, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 200}, []RoadSource{source}, nil, cfg)
	if err != nil {
		t.Fatalf("ComputeReceiverLevels at datum %.0f: %v", datumZ, err)
	}

	return levels
}

// h_m is the mean height of the propagation path above the ground, so a scene
// that is rigidly translated in Z — road surface, receiver ground and receiver
// lifted by the same amount — must radiate exactly the same level. Nothing
// about the geometry changed; only the datum did.
//
// Measuring h_m above sea level instead breaks that: D_gr = 4.8 −
// (2·h_m/s_gr)·(17 + 300/s_gr) goes deeply negative at a site a few hundred
// metres up, clamps to 0, and the ground attenuation silently disappears.
func TestGroundCorrectionIsInvariantUnderVerticalTranslation(t *testing.T) {
	t.Parallel()

	seaLevel := levelsAtDatum(t, 0)
	upland := levelsAtDatum(t, 400)

	if !almostEqual(seaLevel.LrDay, upland.LrDay, 1e-9) {
		t.Fatalf("the same scene 400 m higher is %.3f dB louder by day (%.3f vs %.3f): h_m is measured above sea level, not above the ground",
			upland.LrDay-seaLevel.LrDay, upland.LrDay, seaLevel.LrDay)
	}

	if !almostEqual(seaLevel.LrNight, upland.LrNight, 1e-9) {
		t.Fatalf("the same scene 400 m higher is %.3f dB louder by night (%.3f vs %.3f)",
			upland.LrNight-seaLevel.LrNight, upland.LrNight, seaLevel.LrNight)
	}
}

// pathTerrain is a DTM shaped by a function of position, with an explicit
// extent so that a query can miss.
type pathTerrain struct {
	half      float64
	elevation func(x, y float64) float64
}

func (p pathTerrain) ElevationAt(x, y float64) (float64, bool) {
	if math.Abs(x) > p.half || math.Abs(y) > p.half {
		return 0, false
	}

	return p.elevation(x, y), true
}

func (p pathTerrain) Bounds() [4]float64 {
	return [4]float64{-p.half, -p.half, p.half, p.half}
}

func (p pathTerrain) Info() terrain.Info {
	return terrain.Info{Bounds: p.Bounds(), PixelSize: [2]float64{1, 1}, GridSize: [2]int{1, 1}}
}

// levelsOverTerrain computes the scene of levelsAtDatum at datum 0 with a DTM
// attached, leaving the declared ground elevations where they were.
func levelsOverTerrain(t *testing.T, dtm terrain.Model) PeriodLevels {
	t.Helper()

	cfg := DefaultPropagationConfig()
	cfg.ReceiverHeightM = 4
	cfg.TerrainModel = dtm

	levels, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 200}, []RoadSource{sampleSource()}, nil, cfg)
	if err != nil {
		t.Fatalf("ComputeReceiverLevels over terrain: %v", err)
	}

	return levels
}

// Where no slope edge is declared, the DTM is the ground. A ridge halfway to
// the receiver brings the ground up towards the path, which lowers h_m and
// raises D_gr: the receiver gets quieter, not louder.
func TestGroundCorrectionUsesTheDTMWhereNoSlopeIsDeclared(t *testing.T) {
	t.Parallel()

	flat := levelsOverTerrain(t, pathTerrain{
		half:      1000,
		elevation: func(_, _ float64) float64 { return 0 },
	})

	// A 10 m ridge peaking midway along the 200 m path.
	ridged := levelsOverTerrain(t, pathTerrain{
		half: 1000,
		elevation: func(_, y float64) float64 {
			if y <= 0 || y >= 200 {
				return 0
			}

			return 10 * (1 - math.Abs(y-100)/100)
		},
	})

	if ridged.LrDay >= flat.LrDay {
		t.Fatalf("a ridge between source and receiver left the level at %.3f dB, no lower than flat ground's %.3f dB",
			ridged.LrDay, flat.LrDay)
	}

	if flat.LrDay-ridged.LrDay < 0.2 {
		t.Fatalf("the ridge moved the level by only %.3f dB; the DTM is barely reaching the ground correction",
			flat.LrDay-ridged.LrDay)
	}
}

// The DTM is read relative to the ground already known at each end of the
// path, so a terrain on a different datum than the model's own elevations
// contributes its shape and not its offset. Read absolutely instead, a flat
// DTM 400 m up under a model that puts the road at zero would describe a path
// 400 m underground and invent tens of dB of ground attenuation.
func TestGroundCorrectionReadsTheDTMRelativeToTheDeclaredGround(t *testing.T) {
	t.Parallel()

	withoutTerrain := levelsAtDatum(t, 0)

	overAnotherDatum := levelsOverTerrain(t, pathTerrain{
		half:      1000,
		elevation: func(_, _ float64) float64 { return 400 },
	})

	if !almostEqual(withoutTerrain.LrDay, overAnotherDatum.LrDay, 1e-9) {
		t.Fatalf("a flat DTM on another datum moved the level by %.3f dB (%.3f vs %.3f)",
			overAnotherDatum.LrDay-withoutTerrain.LrDay, overAnotherDatum.LrDay, withoutTerrain.LrDay)
	}
}

// Declared slope edges are surveyed ground and outrank a sampled raster, so a
// DTM that disagrees with them changes nothing.
func TestDeclaredSlopeEdgesOutrankTheDTM(t *testing.T) {
	t.Parallel()

	source := sampleTieflageSource()
	foot := tieflageSlopeFoot()
	crest := tieflageSlopeCrest()
	profile := TerrainProfile{Slopes: []TerrainSlope{{SlopeCrest: crest, SlopeFoot: &foot}}}

	levels := func(dtm terrain.Model) PeriodLevels {
		t.Helper()

		cfg := DefaultPropagationConfig()
		cfg.ReceiverHeightM = 2.8
		cfg.ReceiverTerrainZ = 105.5
		cfg.Terrain = []TerrainProfile{profile}
		cfg.TerrainModel = dtm

		out, err := ComputeReceiverLevels(geo.Point2D{X: 0, Y: 100}, []RoadSource{source}, nil, cfg)
		if err != nil {
			t.Fatalf("ComputeReceiverLevels: %v", err)
		}

		return out
	}

	declaredOnly := levels(nil)
	withDTM := levels(pathTerrain{
		half:      1000,
		elevation: func(_, y float64) float64 { return 105.5 + 30*math.Sin(y/50) },
	})

	if !almostEqual(declaredOnly.LrDay, withDTM.LrDay, 1e-12) {
		t.Fatalf("a contradicting DTM moved a scene with declared slope edges by %.6f dB",
			withDTM.LrDay-declaredOnly.LrDay)
	}
}
