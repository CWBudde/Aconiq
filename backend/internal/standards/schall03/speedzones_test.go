package schall03

import (
	"math"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// tramSegment is a straight 400 m Straßenbahn line below the substitute speed,
// so every speed decision in these tests is driven by the zone split alone.
func tramSegment(features ...TrackFeature) TrackSegment {
	return TrackSegment{
		ID:              "tram",
		TrackCenterline: []geo.Point2D{{X: 0, Y: 0}, {X: 400, Y: 0}},
		StreckeMaxKPH:   25,
		Features:        features,
		Operations: []TrainOperation{{
			TrainType:          "Niederflur-ET",
			FzComposition:      []FzCount{{Fz: 21, Count: 1}},
			SpeedKPH:           25,
			TrainsPerHourDay:   10,
			TrainsPerHourNight: 4,
		}},
	}
}

func haltestelleAt(x float64) TrackFeature {
	return TrackFeature{Kind: TrackFeatureHaltestelle, Point: geo.Point2D{X: x, Y: 0}}
}

func partLengths(t *testing.T, parts []speedZonePart) float64 {
	t.Helper()

	total := 0.0
	for _, part := range parts {
		total += geo.LineStringLength(part.segment.TrackCenterline)
	}

	return total
}

// TestSplitWithoutFeaturesReturnsTheSegmentUntouched is the compatibility
// guarantee: a model that carries no Weichen- or Haltestellen-geometry must go
// through the split byte-for-byte, or every existing golden would move.
func TestSplitWithoutFeaturesReturnsTheSegmentUntouched(t *testing.T) {
	t.Parallel()

	seg := tramSegment()

	parts, err := splitForSpeedZones(seg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}

	if parts[0].substitutionSuppressed {
		t.Fatal("a segment without features must keep the whole-segment substitution")
	}

	if parts[0].segment.ID != seg.ID {
		t.Fatalf("expected the ID to be untouched, got %q", parts[0].segment.ID)
	}

	if len(parts[0].segment.TrackCenterline) != len(seg.TrackCenterline) {
		t.Fatal("expected the centerline to be untouched")
	}
}

func TestSplitPlacesOneZoneAroundAFeature(t *testing.T) {
	t.Parallel()

	parts, err := splitForSpeedZones(tramSegment(haltestelleAt(200)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 0–175 suppressed, 175–225 substituting, 225–400 suppressed.
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}

	wantSuppressed := []bool{true, false, true}
	wantLengths := []float64{175, 50, 175}

	for i, part := range parts {
		if part.substitutionSuppressed != wantSuppressed[i] {
			t.Fatalf("part %d suppressed = %v, want %v", i, part.substitutionSuppressed, wantSuppressed[i])
		}

		got := geo.LineStringLength(part.segment.TrackCenterline)
		if math.Abs(got-wantLengths[i]) > 1e-9 {
			t.Fatalf("part %d length = %.6f, want %.1f", i, got, wantLengths[i])
		}
	}

	if total := partLengths(t, parts); math.Abs(total-400) > 1e-9 {
		t.Fatalf("parts must cover the segment exactly, got %.6f m of 400", total)
	}
}

// TestSplitClipsAZoneAtTheLineEnd pins that a feature at the very start does not
// produce an empty leading part.
func TestSplitClipsAZoneAtTheLineEnd(t *testing.T) {
	t.Parallel()

	parts, err := splitForSpeedZones(tramSegment(haltestelleAt(0)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}

	if parts[0].substitutionSuppressed {
		t.Fatal("the zone at the start must be the substituting part")
	}

	if got := geo.LineStringLength(parts[0].segment.TrackCenterline); math.Abs(got-25) > 1e-9 {
		t.Fatalf("expected the clipped zone to be 25 m, got %.6f", got)
	}
}

// TestSplitMergesOverlappingZones covers two features closer together than
// 50 m, which Nr. 5.3.2 turns into one continuous substituted stretch.
func TestSplitMergesOverlappingZones(t *testing.T) {
	t.Parallel()

	parts, err := splitForSpeedZones(tramSegment(haltestelleAt(200), haltestelleAt(230)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parts) != 3 {
		t.Fatalf("expected the two zones to merge into 3 parts, got %d", len(parts))
	}

	// 175–255 is the union of 175–225 and 205–255.
	if got := geo.LineStringLength(parts[1].segment.TrackCenterline); math.Abs(got-80) > 1e-9 {
		t.Fatalf("expected a merged 80 m zone, got %.6f", got)
	}
}

// TestSplitOrdersUnsortedFeatures pins that the merge does not depend on the
// order features happen to be listed in.
func TestSplitOrdersUnsortedFeatures(t *testing.T) {
	t.Parallel()

	ordered, err := splitForSpeedZones(tramSegment(haltestelleAt(100), haltestelleAt(300)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reversed, err := splitForSpeedZones(tramSegment(haltestelleAt(300), haltestelleAt(100)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ordered) != len(reversed) || len(ordered) != 5 {
		t.Fatalf("expected 5 parts either way, got %d and %d", len(ordered), len(reversed))
	}

	for i := range ordered {
		a := geo.LineStringLength(ordered[i].segment.TrackCenterline)
		b := geo.LineStringLength(reversed[i].segment.TrackCenterline)

		if math.Abs(a-b) > 1e-9 || ordered[i].substitutionSuppressed != reversed[i].substitutionSuppressed {
			t.Fatalf("part %d differs between the two orderings", i)
		}
	}
}

// TestSplitClampsAFeatureJustPastTheEnd covers a Haltestelle sitting just
// beyond the last vertex: it projects onto the end of the line, so its zone
// reaches back into this segment instead of being dropped.  The zone is centred
// on the clamped chainage rather than the true position, which over-states it
// by the overshoot — at most 25 m, and in the conservative direction.
func TestSplitClampsAFeatureJustPastTheEnd(t *testing.T) {
	t.Parallel()

	parts, err := splitForSpeedZones(tramSegment(haltestelleAt(410)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}

	if parts[1].substitutionSuppressed {
		t.Fatal("the clamped zone must be the substituting part")
	}

	if got := geo.LineStringLength(parts[1].segment.TrackCenterline); math.Abs(got-25) > 1e-9 {
		t.Fatalf("expected a 25 m zone at the end, got %.6f", got)
	}

	if total := partLengths(t, parts); math.Abs(total-400) > 1e-9 {
		t.Fatalf("parts must still cover exactly 400 m, got %.6f", total)
	}
}

// TestSplitRejectsAFeatureBeyondTheSegment pins that a feature far past the end
// is refused rather than silently clamped onto a segment it is not on.
func TestSplitRejectsAFeatureBeyondTheSegment(t *testing.T) {
	t.Parallel()

	_, err := splitForSpeedZones(tramSegment(haltestelleAt(500)))
	if err == nil {
		t.Fatal("expected an error for a feature 100 m past the end")
	}

	if !strings.Contains(err.Error(), "from the centerline") {
		t.Fatalf("expected the error to name the offset, got %q", err)
	}
}

func TestSplitRejectsAFeatureOffTheTrack(t *testing.T) {
	t.Parallel()

	seg := tramSegment(TrackFeature{Kind: TrackFeatureWeiche, Point: geo.Point2D{X: 200, Y: 90}})

	_, err := splitForSpeedZones(seg)
	if err == nil {
		t.Fatal("expected an error for a feature off the centerline")
	}

	if !strings.Contains(err.Error(), "from the centerline") {
		t.Fatalf("expected the error to name the offset, got %q", err)
	}
}

func TestTrackFeatureValidateRejectsAnUnknownKind(t *testing.T) {
	t.Parallel()

	err := TrackFeature{Kind: "bahnsteig", Point: geo.Point2D{}}.Validate()
	if err == nil {
		t.Fatal("expected an error for an unknown kind")
	}

	for _, want := range []string{"weiche", "kreuzung", "haltestelle"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected the error to list %q, got %q", want, err)
		}
	}
}

// TestValidateRejectsPermanentlySlowWithFeatures pins the Nr. 5.3.2 exclusion:
// the "dauerhaft v ≤ 30 km/h" exception presupposes a stretch with no Weichen,
// Kreuzungen or Haltestellen on it.
func TestValidateRejectsPermanentlySlowWithFeatures(t *testing.T) {
	t.Parallel()

	seg := tramSegment(haltestelleAt(200))
	seg.PermanentlySlow = true

	err := seg.Validate()
	if err == nil {
		t.Fatal("expected an error for permanently_slow with declared features")
	}

	if !strings.Contains(err.Error(), "permanently_slow") {
		t.Fatalf("expected the error to name permanently_slow, got %q", err)
	}
}

// TestZonedSegmentBracketsTheWholeSegmentReadings is the end-to-end proof that
// the split changes the answer, in the right direction and by a real amount.
// A 25 km/h tram line with one Haltestelle must land strictly between the two
// whole-segment readings that bracket it: below the reading that substitutes
// 50 km/h over the entire segment, which is what Aconiq did before Nr. 5.3.2's
// extent was modelled, and above the 30 km/h permanently-slow reading.
func TestZonedSegmentBracketsTheWholeSegmentReadings(t *testing.T) {
	t.Parallel()

	receiver := ReceiverInput{
		ID:      "r_25m",
		Point:   geo.Point2D{X: 200, Y: 25},
		HeightM: 3.5,
	}

	levelFor := func(t *testing.T, seg TrackSegment) float64 {
		t.Helper()

		levels, err := ComputeNormativeReceiverLevels(receiver, []TrackSegment{seg})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		return levels.LpAeqDay
	}

	// No features: the substitute speed covers the whole segment.
	substitutedThroughout := levelFor(t, tramSegment())

	// One Haltestelle at mid-segment: only the 50 m around it is substituted,
	// and the remaining 350 m run at the real 25 km/h.
	zoned := levelFor(t, tramSegment(haltestelleAt(200)))

	// The Nr. 5.3.2 exception: 30 km/h over the whole segment, no substitution.
	permanentlySlow := tramSegment()
	permanentlySlow.PermanentlySlow = true
	slowThroughout := levelFor(t, permanentlySlow)

	if zoned >= substitutedThroughout {
		t.Fatalf(
			"zoned %.6f dB must be below the whole-segment substitution %.6f dB",
			zoned, substitutedThroughout,
		)
	}

	if zoned <= slowThroughout {
		t.Fatalf(
			"zoned %.6f dB must be above the permanently-slow reading %.6f dB",
			zoned, slowThroughout,
		)
	}

	t.Logf(
		"substituted throughout = %.6f dB, zoned = %.6f dB, permanently slow = %.6f dB",
		substitutedThroughout, zoned, slowThroughout,
	)
}
