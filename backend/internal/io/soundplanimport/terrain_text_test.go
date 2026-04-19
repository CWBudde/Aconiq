package soundplanimport

import (
	"math"
	"os"
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

	if len(terrain.DGMFiles) != 1 {
		t.Fatalf("got %d DGM files, want 1", len(terrain.DGMFiles))
	}

	if terrain.DGMFiles[0].SourceFile != "RDGM0001.dgm" {
		t.Fatalf("DGM source = %q, want RDGM0001.dgm", terrain.DGMFiles[0].SourceFile)
	}

	if len(terrain.DGMFiles[0].Points) != 3672 {
		t.Fatalf("got %d DGM points, want 3672", len(terrain.DGMFiles[0].Points))
	}
}

func TestLoadTerrainDataFallsBackToHoehenTxt(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)
	tmpDir := t.TempDir()

	data, err := os.ReadFile(filepath.Join(dir, "Höhen.txt"))
	if err != nil {
		t.Fatalf("read Höhen.txt: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "Höhen.txt"), data, 0o644); err != nil {
		t.Fatalf("write Höhen.txt: %v", err)
	}

	terrain, err := LoadTerrainData(tmpDir)
	if err != nil {
		t.Fatalf("LoadTerrainData: %v", err)
	}

	if len(terrain.ElevationPoints) < 26000 {
		t.Fatalf("got %d elevation points, want at least 26000 from Höhen.txt", len(terrain.ElevationPoints))
	}

	if len(terrain.ContourLines) != 0 {
		t.Fatalf("got %d contour lines, want 0 without GeoTmp.geo", len(terrain.ContourLines))
	}

	if len(terrain.DGMFiles) != 0 {
		t.Fatalf("got %d DGM files, want 0 in text-only fallback", len(terrain.DGMFiles))
	}
}
