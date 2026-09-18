package schall03

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/terrain"
)

// These tests cover the second half of the ground story. ground_datum_test.go
// pins the *datum*: which plane a height is measured from. This file pins the
// *profile*: that Gl. 15's h_m follows the terrain between source and receiver
// instead of assuming one level plane under the whole path.
//
// Gl. 14 is the reason the sign matters:
//
//	A_gr,B = max(4.8 − (2·h_m/d)·(17 + 300·d₀/d), 0)
//
// A_gr,B is an attenuation, subtracted from the level. h_m appears with a
// negative sign, so a *larger* h_m means *less* ground attenuation and a
// *louder* receiver, until the bracket goes negative and the ≥ 0 dB clamp
// erases the Bodendämpfung entirely (up to 4.8 dB). Ground that rises under the
// path lifts the mean ground towards the path, shrinks h_m and makes the
// receiver quieter; ground that falls away does the opposite.

// profileTerrain is a terrain model defined by a function rather than a raster,
// so a slope, a ridge or a valley can be stated exactly and the test asserts
// against arithmetic instead of against an interpolated grid.
type profileTerrain struct {
	bounds [4]float64 // minX, minY, maxX, maxY
	z      func(x, y float64) float64
}

func (p profileTerrain) ElevationAt(x, y float64) (float64, bool) {
	if x < p.bounds[0] || x > p.bounds[2] || y < p.bounds[1] || y > p.bounds[3] {
		return 0, false
	}

	return p.z(x, y), true
}

func (p profileTerrain) Bounds() [4]float64 { return p.bounds }

func (p profileTerrain) Info() terrain.Info {
	return terrain.Info{Bounds: p.bounds, PixelSize: [2]float64{1, 1}, GridSize: [2]int{1, 1}}
}

// groundProfileScene is the geometry every case below shares: a subsegment
// midpoint on the track at the origin and a receiver 200 m abeam of it, with
// the receiver standing on ground at receiverGroundZ.
func groundProfileScene(receiverGroundZ float64) (geo.Point2D, ReceiverInput) {
	source := geo.Point2D{X: 0, Y: 0}
	receiver := ReceiverInput{
		ID:       "r1",
		Point:    geo.Point2D{X: 0, Y: 200},
		HeightM:  3.5,
		TerrainZ: receiverGroundZ,
	}

	return source, receiver
}

// TestGroundOffsetIsExactlyZeroWithoutTerrain is the identity that makes the
// whole change safe to land: with no terrain model the ground term is the
// literal zero, so h_m is (h_g + h_r)/2 — bit for bit the value the
// flat-ground reading computed before any of this existed.
//
// Equality is exact on purpose. An "almost zero" offset would move every
// existing result by a few ULP and put every golden snapshot in play.
func TestGroundOffsetIsExactlyZeroWithoutTerrain(t *testing.T) {
	t.Parallel()

	source, receiver := groundProfileScene(137.5)

	offset := resolvePathGroundOffset(nil, source, receiver)
	if offset != 0 {
		t.Fatalf("resolvePathGroundOffset with no terrain = %g, want exactly 0", offset)
	}

	// The flat-ground expression, written out rather than called, so that a
	// future edit to meanPathHeight has to reproduce it rather than agree with
	// itself.
	for _, hg := range []float64{0, 0.25, 4, 5, 12.5, 400} {
		for _, hr := range []float64{0, 3.5, 7, 22.5} {
			want := (hg + hr) / 2

			got := meanPathHeight(hg, hr, offset)
			if got != want {
				t.Errorf("meanPathHeight(%g, %g, 0) = %.20g, want exactly %.20g", hg, hr, got, want)
			}
		}
	}
}

// TestUniformSlopeMovesTheGroundThoughTheRiseIsZero is the case that proves the
// correction is more than a rise-above-chord term.
//
// Over a uniform slope every interior sample lies exactly on the chord between
// the two endpoint elevations, so terrain.MeanRiseAboveChord returns 0 and a
// rise-only implementation would compute the flat-ground value. The mean ground
// has moved all the same: it is the chord itself, halfway between the ground at
// the source and the ground at the receiver.
func TestUniformSlopeMovesTheGroundThoughTheRiseIsZero(t *testing.T) {
	t.Parallel()

	// Ground falling 1 m per 10 m in +y: 40 m at the track, 20 m at the
	// receiver 200 m away.
	const (
		sourceGroundZ   = 40.0
		receiverGroundZ = 20.0
	)

	slope := profileTerrain{
		bounds: [4]float64{-1000, -1000, 1000, 1000},
		z:      func(_, y float64) float64 { return sourceGroundZ - y/10 },
	}

	source, receiver := groundProfileScene(receiverGroundZ)

	rise, ok := terrain.MeanRiseAboveChord(slope, source.X, source.Y, receiver.Point.X, receiver.Point.Y, terrainSampleStepM)
	if !ok {
		t.Fatal("MeanRiseAboveChord did not resolve a path wholly inside the terrain")
	}

	if math.Abs(rise) > 1e-9 {
		t.Fatalf("a uniform slope has no rise above its own chord, got %g m", rise)
	}

	offset := resolvePathGroundOffset(slope, source, receiver)

	want := (sourceGroundZ - receiverGroundZ) / 2
	if math.Abs(offset-want) > 1e-9 {
		t.Fatalf("ground offset = %g m, want %g m (the chord, halfway between the two ends)", offset, want)
	}

	// The whole point: with the rise alone the offset would be 0 and h_m would
	// keep the flat-plane value.
	const (
		hg = 4.0 // pantograph Teilquelle above the reference plane
		hr = 3.5
	)

	flat := meanPathHeight(hg, hr, 0)

	sloped := meanPathHeight(hg, hr, offset)
	if sloped >= flat {
		t.Fatalf("h_m over the slope (%g m) did not drop below the flat-ground value (%g m)", sloped, flat)
	}

	// Gl. 14: a smaller h_m is more ground attenuation, so the slope makes this
	// receiver quieter.
	const d = 200.0

	if agrB(sloped, d) <= agrB(flat, d) {
		t.Fatalf("A_gr,B over the slope (%g dB) is not greater than over flat ground (%g dB)", agrB(sloped, d), agrB(flat, d))
	}
}

// TestRidgeAndValleyMoveAgrBInOppositeDirections pins the sign of the
// correction against Gl. 14 rather than against a recorded number.
//
// A ridge between source and receiver lifts the mean ground towards the
// propagation path. h_m = (mean path height) − (mean ground) therefore shrinks,
// the term 2·h_m/d in Gl. 14 shrinks with it, and A_gr,B — an attenuation —
// grows: the receiver gets quieter. A valley lowers the mean ground, h_m grows,
// A_gr,B falls towards its ≥ 0 dB floor and the receiver gets louder. That
// floor is the failure mode the flat-ground reading could reach at any site
// with relief, and it caps the error at the full 4.8 dB of Gl. 14.
func TestRidgeAndValleyMoveAgrBInOppositeDirections(t *testing.T) {
	t.Parallel()

	const (
		groundZ = 10.0
		relief  = 12.0 // crest height of the ridge / depth of the valley, in m
		hg      = 4.0
		hr      = 3.5
		d       = 200.0
	)

	// A triangular feature spanning the path, peaking halfway between track and
	// receiver, and level with both ends so that the endpoint elevations — and
	// with them the chord — are identical in all three cases. Only the rise
	// differs, which isolates exactly the term under test.
	feature := func(peak float64) profileTerrain {
		return profileTerrain{
			bounds: [4]float64{-1000, -1000, 1000, 1000},
			z: func(_, y float64) float64 {
				if y <= 0 || y >= 200 {
					return groundZ
				}

				return groundZ + peak*(1-math.Abs(y-100)/100)
			},
		}
	}

	source, receiver := groundProfileScene(groundZ)

	flat := meanPathHeight(hg, hr, resolvePathGroundOffset(feature(0), source, receiver))
	ridge := meanPathHeight(hg, hr, resolvePathGroundOffset(feature(relief), source, receiver))
	valley := meanPathHeight(hg, hr, resolvePathGroundOffset(feature(-relief), source, receiver))

	if ridge >= flat {
		t.Errorf("h_m over a ridge (%g m) is not below the level-ground value (%g m)", ridge, flat)
	}

	if valley <= flat {
		t.Errorf("h_m over a valley (%g m) is not above the level-ground value (%g m)", valley, flat)
	}

	if agrB(ridge, d) <= agrB(flat, d) {
		t.Errorf("Gl. 14: a ridge must add ground attenuation, got %g dB against %g dB", agrB(ridge, d), agrB(flat, d))
	}

	if agrB(valley, d) >= agrB(flat, d) {
		t.Errorf("Gl. 14: a valley must remove ground attenuation, got %g dB against %g dB", agrB(valley, d), agrB(flat, d))
	}

	t.Logf("A_gr,B: ridge %.4f dB, level %.4f dB, valley %.4f dB", agrB(ridge, d), agrB(flat, d), agrB(valley, d))
}

// TestHmNeverGoesNegativeUnderGroundThatRisesAboveThePath checks the floor
// meanPathHeight applies. Ground whose mean lies above the straight
// source→receiver path — a hill the path would have to pass through, which the
// Abschirmung of Nr. 6.5 and not Gl. 14 is there to describe — drives
// (h_g + h_r)/2 − groundOffset negative. Gl. 14 has no branch for that, and
// extrapolating one would grow A_gr,B without limit, so h_m stops at 0.
func TestHmNeverGoesNegativeUnderGroundThatRisesAboveThePath(t *testing.T) {
	t.Parallel()

	hill := profileTerrain{
		bounds: [4]float64{-1000, -1000, 1000, 1000},
		z: func(_, y float64) float64 {
			if y <= 0 || y >= 200 {
				return 0
			}

			return 80
		},
	}

	source, receiver := groundProfileScene(0)

	offset := resolvePathGroundOffset(hill, source, receiver)
	if offset <= 4 {
		t.Fatalf("ground offset = %g m; the hill does not rise above the path and the case covers nothing", offset)
	}

	hm := meanPathHeight(4, 3.5, offset)
	if hm != 0 {
		t.Fatalf("h_m = %g m under a hill that rises above the path, want the 0 m floor", hm)
	}
}

// TestAPathOutsideTheTerrainFallsBackToTheFlatPlane states the rule PLAN.md
// §1.5 and §1.8 both state: a table cell that does not exist is not a zero.
//
// A terrain that does not reach this path must leave it exactly where it was —
// on the receiver's own ground plane — rather than reading the missing samples
// as an elevation of 0, which at a site 137 m up would put the path 137 m in
// the air and clamp the Bodendämpfung away.
func TestAPathOutsideTheTerrainFallsBackToTheFlatPlane(t *testing.T) {
	t.Parallel()

	const groundZ = 137.0

	// A DTM 10 km east of the scene — the shape a CRS or extent mistake takes.
	elsewhere := profileTerrain{
		bounds: [4]float64{10000, 10000, 11000, 11000},
		z:      func(_, _ float64) float64 { return 0 },
	}

	source, receiver := groundProfileScene(groundZ)

	offset := resolvePathGroundOffset(elsewhere, source, receiver)
	if offset != 0 {
		t.Fatalf("ground offset = %.20g for a path the terrain does not reach, want exactly 0", offset)
	}

	// And through the whole chain: a scene carrying that terrain computes bit
	// for bit what the same scene with no terrain at all computes.
	segment, sceneReceiver := testScene(t, groundZ)

	withoutTerrain, err := ComputeNormativeReceiverLevelsForScene(sceneReceiver, NormativeScene{
		Segments: []TrackSegment{segment},
	})
	if err != nil {
		t.Fatalf("compute without terrain: %v", err)
	}

	withMissingTerrain, err := ComputeNormativeReceiverLevelsForScene(sceneReceiver, NormativeScene{
		Segments: []TrackSegment{segment},
		Terrain:  elsewhere,
	})
	if err != nil {
		t.Fatalf("compute with out-of-extent terrain: %v", err)
	}

	if withMissingTerrain != withoutTerrain {
		t.Fatalf("an out-of-extent terrain changed the levels:\n without: %+v\n with:    %+v",
			withoutTerrain, withMissingTerrain)
	}
}

// TestNilTerrainSceneIsBitIdenticalToTheFlatGroundEntryPoints is the wrapper
// check: the four older entry points are now thin wrappers over
// ComputeNormativeReceiverLevelsForScene, and they must still compute the
// flat-ground result exactly.
func TestNilTerrainSceneIsBitIdenticalToTheFlatGroundEntryPoints(t *testing.T) {
	t.Parallel()

	walls := []ReflectingWall{
		{A: geo.Point2D{X: -500, Y: -15}, B: geo.Point2D{X: 500, Y: -15}, HeightM: 16, Surface: WallSurfaceHard},
	}
	barriers := []BarrierSegment{
		{A: geo.Point2D{X: -500, Y: 100}, B: geo.Point2D{X: 500, Y: 100}, TopHeightM: 4},
	}

	segment, receiver := testScene(t, 250)
	segments := []TrackSegment{segment}

	scene, err := ComputeNormativeReceiverLevelsForScene(receiver, NormativeScene{
		Segments: segments,
		Walls:    walls,
		Barriers: barriers,
	})
	if err != nil {
		t.Fatalf("compute for scene: %v", err)
	}

	legacy, err := ComputeNormativeReceiverLevelsWithScene(receiver, segments, walls, barriers)
	if err != nil {
		t.Fatalf("compute with scene: %v", err)
	}

	if scene != legacy {
		t.Fatalf("the wrapper disagrees with the scene entry point:\n wrapper: %+v\n scene:   %+v", legacy, scene)
	}
}
