package schall03

import (
	"math"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// The defect these tests pin: elevation_m is an absolute Z (the SoundPLAN
// import writes the track's ZTrack straight into it), while a receiver's
// height_m is a height above the ground it stands on. The propagation chain
// used to subtract one from the other and feed the result to Gl. 9, Gl. 11,
// Gl. 12 and Gl. 14/15 as though both were the same kind of number.
//
// A correct implementation is invariant under a rigid vertical translation of
// the whole scene: moving a track, a receiver and the ground under it 400 m up
// changes nothing anyone can hear.

// translationM is the vertical shift the scene is tested against — large
// enough that every affected term moves by far more than any tolerance, and
// realistic for an Alpine or Mittelgebirge site.
const translationM = 400.0

// testScene returns a straight track with one receiver 200 m abeam of it,
// placed on ground at the given absolute elevation. 200 m is chosen because
// Gl. 14's A_gr,B is comfortably non-zero there: closer in, the ≥ 0 dB clamp
// fires for a correct h_m too and would hide the very term under test.
func testScene(t *testing.T, groundZ float64) (TrackSegment, ReceiverInput) {
	t.Helper()

	op, err := NewTrainOperationFromZugart("ICE-1-Zug", 4, 2)
	if err != nil {
		t.Fatalf("NewTrainOperationFromZugart: %v", err)
	}

	segment := TrackSegment{
		ID:              "seg1",
		TrackCenterline: []geo.Point2D{{X: -500, Y: 0}, {X: 500, Y: 0}},
		ElevationM:      groundZ, // Schienenoberkante on the ground plane
		Fahrbahn:        FahrbahnartSchwellengleis,
		Surface:         SurfaceCondNone,
		StreckeMaxKPH:   250,
		Operations:      []TrainOperation{*op},
	}

	receiver := ReceiverInput{
		ID:       "r1",
		Point:    geo.Point2D{X: 0, Y: 200},
		HeightM:  3.5,
		TerrainZ: groundZ,
	}

	return segment, receiver
}

// TestPathGeometryIsInvariantUnderVerticalTranslation checks the three terms
// the defect reached — h_m (Gl. 15, and through it A_gr,B), d (Gl. 11/12) and
// D_Ω (Gl. 9) — one by one rather than only through the receiver level. The
// two errors pull in opposite directions and partly cancel, so a level that
// barely moves is not evidence that the geometry is right.
func TestPathGeometryIsInvariantUnderVerticalTranslation(t *testing.T) {
	t.Parallel()

	const dp = 200.0

	const receiverHeightM = 3.5

	for _, h := range teilquelleHeightIndices {
		atSeaLevel := newPathGeometry(heightAboveSO[h], 0, receiverHeightM, dp, 0)
		translated := newPathGeometry(translationM+heightAboveSO[h], translationM, receiverHeightM, dp, 0)

		if translated != atSeaLevel {
			t.Errorf("h=%d: translating the scene %g m changed the path geometry:\n at sea level: %+v\n translated:  %+v",
				h, translationM, atSeaLevel, translated)
		}

		if agrB(translated.MeanHeightM, translated.SlantDistanceM) != agrB(atSeaLevel.MeanHeightM, atSeaLevel.SlantDistanceM) {
			t.Errorf("h=%d: A_gr,B moved under translation", h)
		}
	}
}

// TestPathGeometryRejectsTheAbsoluteElevationAsAHeight records what the defect
// did to each component, so that a future change which reintroduces the datum
// mix fails here with the numbers rather than with a vague level shift.
//
// The "defective" column is the old code verbatim: h_g = elevation_m +
// h_above_SO with no ground subtracted, h_r = receiver height above ground.
func TestPathGeometryRejectsTheAbsoluteElevationAsAHeight(t *testing.T) {
	t.Parallel()

	const (
		dp              = 200.0
		receiverHeightM = 3.5
		h               = 2 // the pantograph Teilquelle, 4 m above Schienenoberkante
	)

	correct := newPathGeometry(translationM+heightAboveSO[h], translationM, receiverHeightM, dp, 0)

	// The old expression, reproduced here rather than called, because the
	// production code no longer has a way to say it.
	defectiveHg := translationM + heightAboveSO[h]
	defectiveHm := meanPathHeight(defectiveHg, receiverHeightM, 0)
	defectiveD := math.Sqrt(dp*dp + (defectiveHg-receiverHeightM)*(defectiveHg-receiverHeightM))

	checks := []struct {
		name            string
		got, defective  float64
		wantTolerance   float64
		wantApproximate float64
	}{
		{"h_m", correct.MeanHeightM, defectiveHm, 1e-9, 3.75},
		{"d", correct.SlantDistanceM, defectiveD, 1e-3, 200.0006},
		{
			"A_gr,B",
			agrB(correct.MeanHeightM, correct.SlantDistanceM),
			agrB(defectiveHm, defectiveD),
			1e-3,
			4.1063,
		},
	}

	for _, check := range checks {
		if math.Abs(check.got-check.wantApproximate) > check.wantTolerance {
			t.Errorf("%s = %g, want %g (the value the same scene has at sea level)",
				check.name, check.got, check.wantApproximate)
		}

		if math.Abs(check.got-check.defective) < check.wantTolerance {
			t.Errorf("%s: the defective and the corrected value agree (%g); the test no longer reproduces anything",
				check.name, check.got)
		}

		t.Logf("%s: corrected %.4f, mixed datum %.4f", check.name, check.got, check.defective)
	}
}

// TestNormativeLevelsAreInvariantUnderVerticalTranslation is the whole-scene
// statement of the same property, through every public entry point of the
// normative chain including reflection and shielding. Equality is exact: the
// two scenes differ only by a constant that must cancel out of every term.
func TestNormativeLevelsAreInvariantUnderVerticalTranslation(t *testing.T) {
	t.Parallel()

	barriers := []BarrierSegment{
		{A: geo.Point2D{X: -500, Y: 100}, B: geo.Point2D{X: 500, Y: 100}, TopHeightM: 4},
	}
	walls := []ReflectingWall{
		{A: geo.Point2D{X: -500, Y: -15}, B: geo.Point2D{X: 500, Y: -15}, HeightM: 16, Surface: WallSurfaceHard},
	}

	cases := []struct {
		name string
		// freeField marks the baseline case. Every other case must differ from
		// it, or the wall and the barrier are in the scene without taking part
		// in it and the case proves nothing about the reflected and diffracted
		// paths it claims to cover.
		freeField bool
		compute   func(ReceiverInput, []TrackSegment) (NormativeReceiverLevels, error)
	}{
		{
			name:      "free field",
			freeField: true,
			compute:   ComputeNormativeReceiverLevels,
		},
		{
			name: "with walls",
			compute: func(r ReceiverInput, s []TrackSegment) (NormativeReceiverLevels, error) {
				return ComputeNormativeReceiverLevelsWithWalls(r, s, walls)
			},
		},
		{
			name: "with walls and barriers",
			compute: func(r ReceiverInput, s []TrackSegment) (NormativeReceiverLevels, error) {
				return ComputeNormativeReceiverLevelsWithScene(r, s, walls, barriers)
			},
		},
	}

	seaSegment, seaReceiver := testScene(t, 0)

	freeField, err := ComputeNormativeReceiverLevels(seaReceiver, []TrackSegment{seaSegment})
	if err != nil {
		t.Fatalf("compute free field: %v", err)
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			segment, receiver := testScene(t, 0)

			atSeaLevel, err := testCase.compute(receiver, []TrackSegment{segment})
			if err != nil {
				t.Fatalf("compute at sea level: %v", err)
			}

			if !testCase.freeField && atSeaLevel == freeField {
				t.Fatalf("the scene geometry made no difference (%.6f dB); this case covers nothing", atSeaLevel.LrDay)
			}

			highSegment, highReceiver := testScene(t, translationM)

			translated, err := testCase.compute(highReceiver, []TrackSegment{highSegment})
			if err != nil {
				t.Fatalf("compute translated: %v", err)
			}

			if translated != atSeaLevel {
				t.Fatalf("translating the scene %g m moved the levels:\n at sea level: %+v\n translated:  %+v",
					translationM, atSeaLevel, translated)
			}
		})
	}
}

// TestNormativeLevelsStillMoveWhenTheGroundIsNotDeclared is the counterpart:
// it shows that the invariance above is bought with the ground elevation, not
// with an accident of the formulas. A track lifted to an absolute 400 m over a
// receiver still standing on ground at Z = 0 describes a viaduct, and it is
// supposed to read differently.
func TestNormativeLevelsStillMoveWhenTheGroundIsNotDeclared(t *testing.T) {
	t.Parallel()

	seaSegment, seaReceiver := testScene(t, 0)

	atSeaLevel, err := ComputeNormativeReceiverLevels(seaReceiver, []TrackSegment{seaSegment})
	if err != nil {
		t.Fatalf("compute at sea level: %v", err)
	}

	viaduct := seaSegment
	viaduct.ElevationM = translationM

	lifted, err := ComputeNormativeReceiverLevels(seaReceiver, []TrackSegment{viaduct})
	if err != nil {
		t.Fatalf("compute lifted: %v", err)
	}

	if lifted == atSeaLevel {
		t.Fatal("lifting the track 400 m above the receiver's ground changed nothing")
	}

	t.Logf("ground at Z=0, track at Z=%g: L_r,Tag %.4f vs %.4f (%.4f dB)",
		translationM, atSeaLevel.LrDay, lifted.LrDay, lifted.LrDay-atSeaLevel.LrDay)
}
