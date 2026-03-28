package soundplanimport

import (
	"math"
	"path/filepath"
	"testing"
)

func TestParseHoehenTxtFile(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	points, err := ParseHoehenTxtFile(filepath.Join(dir, "Höhen.txt"))
	if err != nil {
		t.Fatalf("ParseHoehenTxtFile: %v", err)
	}

	if len(points) < 26000 {
		t.Fatalf("got %d elevation points, want at least 26000", len(points))
	}

	first := points[0]
	if math.Abs(first.X-7870.52) > 0.01 {
		t.Errorf("first X = %.2f, want 7870.52", first.X)
	}

	if math.Abs(first.Y-6349.22) > 0.01 {
		t.Errorf("first Y = %.2f, want 6349.22", first.Y)
	}

	if math.Abs(first.Z-222.61) > 0.01 {
		t.Errorf("first Z = %.2f, want 222.61", first.Z)
	}
}

func TestLoadTerrainDataPrefersGeoTmp(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	terrain, err := LoadTerrainData(dir)
	if err != nil {
		t.Fatalf("LoadTerrainData: %v", err)
	}

	if len(terrain.ElevationPoints) != 26603 {
		t.Fatalf("got %d elevation points, want 26603 from GeoTmp.geo", len(terrain.ElevationPoints))
	}

	if len(terrain.ContourLines) != 7 {
		t.Fatalf("got %d contour lines, want 7", len(terrain.ContourLines))
	}
}
