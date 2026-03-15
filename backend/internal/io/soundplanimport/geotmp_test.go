package soundplanimport

import (
	"path/filepath"
	"testing"
)

func TestParseGeoTmp_ElevationPointCount(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	terrain, err := ParseGeoTmpFile(filepath.Join(dir, "GeoTmp.geo"))
	if err != nil {
		t.Fatalf("ParseGeoTmpFile: %v", err)
	}

	// The sample project has 26603 elevation points (type 0x040b).
	if len(terrain.ElevationPoints) != 26603 {
		t.Errorf("got %d elevation points, want 26603", len(terrain.ElevationPoints))
	}
}

func TestParseGeoTmp_ContourLineCount(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	terrain, err := ParseGeoTmpFile(filepath.Join(dir, "GeoTmp.geo"))
	if err != nil {
		t.Fatalf("ParseGeoTmpFile: %v", err)
	}

	// The sample project has 4 contour lines (0x040a) + 3 terrain lines (0x046e) = 7.
	if len(terrain.ContourLines) != 7 {
		t.Errorf("got %d contour lines, want 7", len(terrain.ContourLines))
	}
}

func TestParseGeoTmp_ElevationCoordinatesPlausible(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	terrain, err := ParseGeoTmpFile(filepath.Join(dir, "GeoTmp.geo"))
	if err != nil {
		t.Fatalf("ParseGeoTmpFile: %v", err)
	}

	for i, pt := range terrain.ElevationPoints {
		if pt.X < 5000 || pt.X > 10000 || pt.Y < 5000 || pt.Y > 8000 {
			t.Errorf("elev point %d: (%.2f,%.2f) out of expected range", i, pt.X, pt.Y)

			break
		}

		if pt.Z < 150 || pt.Z > 350 {
			t.Errorf("elev point %d: Z=%.2f out of expected range [150,350]", i, pt.Z)

			break
		}
	}
}

func TestParseGeoTmp_ContourLinesHaveMultiplePoints(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	terrain, err := ParseGeoTmpFile(filepath.Join(dir, "GeoTmp.geo"))
	if err != nil {
		t.Fatalf("ParseGeoTmpFile: %v", err)
	}

	for i, cl := range terrain.ContourLines {
		if len(cl.Points) < 3 {
			t.Errorf("contour line %d: only %d points, want >= 3", i, len(cl.Points))
		}
	}
}

func TestParseGeoTmp_FirstElevationPointMatchesHoehenTxt(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	terrain, err := ParseGeoTmpFile(filepath.Join(dir, "GeoTmp.geo"))
	if err != nil {
		t.Fatalf("ParseGeoTmpFile: %v", err)
	}

	// Höhen.txt first line: 7870,52; 6349,22; 222,61
	// Elevation points are from this data. Check that a point near these
	// coordinates exists (order may differ from text file).
	found := false

	for _, pt := range terrain.ElevationPoints {
		dx := pt.X - 7870.52
		dy := pt.Y - 6349.22

		if dx*dx+dy*dy < 1.0 {
			found = true

			if pt.Z < 222 || pt.Z > 223 {
				t.Errorf("point near (7870.52,6349.22): Z=%.2f, want ~222.61", pt.Z)
			}

			break
		}
	}

	if !found {
		t.Error("no elevation point found near (7870.52, 6349.22) from Höhen.txt")
	}
}
