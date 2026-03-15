package soundplanimport

import (
	"math"
	"path/filepath"
	"testing"
)

func TestParseGeoRail_TrackCount(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	tracks, err := ParseGeoRailFile(filepath.Join(dir, "GeoRail.geo"))
	if err != nil {
		t.Fatalf("ParseGeoRailFile: %v", err)
	}

	// The sample project has "Hauptstrecke Gleis 1" and "Hauptstrecke Gleis 2".
	if len(tracks) != 2 {
		t.Fatalf("got %d tracks, want 2", len(tracks))
	}
}

func TestParseGeoRail_TrackNames(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	tracks, err := ParseGeoRailFile(filepath.Join(dir, "GeoRail.geo"))
	if err != nil {
		t.Fatalf("ParseGeoRailFile: %v", err)
	}

	want := []string{"Hauptstrecke Gleis 1", "Hauptstrecke Gleis 2"}

	for i, w := range want {
		if tracks[i].Name != w {
			t.Errorf("tracks[%d].Name = %q, want %q", i, tracks[i].Name, w)
		}
	}
}

func TestParseGeoRail_CoordinatesPlausible(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	tracks, err := ParseGeoRailFile(filepath.Join(dir, "GeoRail.geo"))
	if err != nil {
		t.Fatalf("ParseGeoRailFile: %v", err)
	}

	// All coordinates should be in the local system range.
	for ti, track := range tracks {
		for si, seg := range track.Segments {
			for pi, pt := range seg.Points {
				if pt.X < 6000 || pt.X > 9000 {
					t.Errorf("track %d seg %d pt %d: X=%.2f out of range [6000,9000]", ti, si, pi, pt.X)
				}

				if pt.Y < 6000 || pt.Y > 8000 {
					t.Errorf("track %d seg %d pt %d: Y=%.2f out of range [6000,8000]", ti, si, pi, pt.Y)
				}

				if pt.ZTrack < 200 || pt.ZTrack > 300 {
					t.Errorf("track %d seg %d pt %d: ZTrack=%.2f out of range [200,300]", ti, si, pi, pt.ZTrack)
				}
			}
		}
	}
}

func TestParseGeoRail_SegmentsHavePoints(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	tracks, err := ParseGeoRailFile(filepath.Join(dir, "GeoRail.geo"))
	if err != nil {
		t.Fatalf("ParseGeoRailFile: %v", err)
	}

	totalPoints := 0

	for _, track := range tracks {
		if len(track.Segments) == 0 {
			t.Errorf("track %q has no segments", track.Name)
		}

		for _, seg := range track.Segments {
			totalPoints += len(seg.Points)
		}
	}

	// From the hex dump we counted ~16 :G  records per track.
	if totalPoints < 10 {
		t.Errorf("total points = %d, want >= 10", totalPoints)
	}
}

func TestParseGeoRail_SpeedExtracted(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	tracks, err := ParseGeoRailFile(filepath.Join(dir, "GeoRail.geo"))
	if err != nil {
		t.Fatalf("ParseGeoRailFile: %v", err)
	}

	// The first segment of each track should have a positive speed.
	for _, track := range tracks {
		if len(track.Segments) == 0 {
			continue
		}

		speed := track.Segments[0].Params.Speed

		if speed <= 0 || speed > 300 {
			t.Errorf("track %q segment 0: Speed=%.2f, want reasonable value", track.Name, speed)
		}
	}
}

func TestParseGeoRail_BridgeSentinel(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	tracks, err := ParseGeoRailFile(filepath.Join(dir, "GeoRail.geo"))
	if err != nil {
		t.Fatalf("ParseGeoRailFile: %v", err)
	}

	// At least one segment should have the -1000 bridge sentinel (no bridge).
	hasSentinel := false

	for _, track := range tracks {
		for _, seg := range track.Segments {
			if seg.Params.BridgeCorrection == -1000.0 {
				hasSentinel = true
			}
		}
	}

	if !hasSentinel {
		t.Error("no segment with BridgeCorrection=-1000 (no-bridge sentinel) found")
	}
}

func TestParseGeoRail_FirstPointOfGleis1(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	tracks, err := ParseGeoRailFile(filepath.Join(dir, "GeoRail.geo"))
	if err != nil {
		t.Fatalf("ParseGeoRailFile: %v", err)
	}

	// First point of Gleis 1 from hex decode: x=7978.65, y=6774.81.
	pt := tracks[0].Segments[0].Points[0]

	if math.Abs(pt.X-7978.65) > 0.01 {
		t.Errorf("first point X=%.2f, want ~7978.65", pt.X)
	}

	if math.Abs(pt.Y-6774.81) > 0.01 {
		t.Errorf("first point Y=%.2f, want ~6774.81", pt.Y)
	}
}
