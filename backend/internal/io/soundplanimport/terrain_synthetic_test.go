package soundplanimport

import (
	"path/filepath"
	"strings"
	"testing"
)

// Fixture-free tests for the Höhen.txt reader and for the terrain loader that
// chooses between the binary and the text source.

// TestParseHoehenTxtFile_Synthetic pins the text elevation reader: German
// decimals, surplus columns, and lines that carry only whitespace.
func TestParseHoehenTxtFile_Synthetic(t *testing.T) {
	t.Parallel()

	content := "7870,52;6349,22;222,61\n" +
		"\n" +
		"  7880,52 ; 6349,22 ; 223,10 ; ignored ; columns  \n" +
		"7890.52;6349.22;224\n" +
		"   \n"

	points, err := ParseHoehenTxtFile(writeFixture(t, t.TempDir(), "Höhen.txt", []byte(content)))
	if err != nil {
		t.Fatalf("ParseHoehenTxtFile: %v", err)
	}

	want := []ElevationPoint{
		{X: 7870.52, Y: 6349.22, Z: 222.61},
		{X: 7880.52, Y: 6349.22, Z: 223.10},
		{X: 7890.52, Y: 6349.22, Z: 224},
	}

	if len(points) != len(want) {
		t.Fatalf("got %d points, want %d (blank lines must be skipped)", len(points), len(want))
	}

	for i, expected := range want {
		if points[i] != expected {
			t.Errorf("points[%d] = %+v, want %+v", i, points[i], expected)
		}
	}
}

// TestParseHoehenTxtFile_Refusals pins the reader's failures. A line with too
// few fields names its own line number, because the file has tens of thousands
// of them and "somewhere in Höhen.txt" is not an actionable message.
func TestParseHoehenTxtFile_Refusals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		wantText string
	}{
		{
			name:     "a line with two fields",
			content:  "1,0;2,0;3,0\n4,0;5,0\n",
			wantText: "line 2",
		},
		{
			name:     "a header line",
			content:  "X Y Z\n1,0;2,0;3,0\n",
			wantText: "line 1",
		},
		{
			name:     "a file with no data lines",
			content:  "\n   \n",
			wantText: "no elevation points found",
		},
		{
			name:     "an empty file",
			content:  "",
			wantText: "no elevation points found",
		},
		{
			name:     "a line past the scanner buffer limit",
			content:  strings.Repeat("9", 1024*1024+1) + ";1;2\n",
			wantText: "scan hoehen.txt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseHoehenTxtFile(writeFixture(t, t.TempDir(), "Höhen.txt", []byte(tc.content)))
			if err == nil {
				t.Fatal("got nil error, want a refusal")
			}

			if !strings.Contains(err.Error(), tc.wantText) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantText)
			}
		})
	}
}

// TestParseHoehenTxtFile_MissingFile pins that an absent file is an error, not
// an empty point list: LoadTerrainData decides its fallback on it.
func TestParseHoehenTxtFile_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := ParseHoehenTxtFile(filepath.Join(t.TempDir(), "Höhen.txt"))
	if err == nil {
		t.Fatal("got nil error for a missing Höhen.txt")
	}
}

// syntheticGeoTmp builds a GeoTmp.geo image with one elevation sample and one
// contour line, which is enough to count as binary terrain.
func syntheticGeoTmp() []byte {
	return concat(
		geoObjectRecord(wireTypeElevPoint, 0),
		geoPointRecord(10, 20, 200, 0),
		geoObjectRecord(wireTypeContour, 0),
		geoPointRecord(0, 0, 195, 0),
		geoPointRecord(10, 0, 195, 0),
	)
}

// TestLoadTerrainData_SourceSelection pins which terrain source wins, which is
// the whole point of the loader: GeoTmp.geo when it carries features, and the
// Höhen.txt text export only when it does not.
func TestLoadTerrainData_SourceSelection(t *testing.T) {
	t.Parallel()

	const hoehen = "500,0;600,0;250,0\n501,0;600,0;250,5\n"

	tests := []struct {
		name       string
		geoTmp     []byte
		hoehenText string
		wantPoints int
		wantLines  int
	}{
		{
			name:       "GeoTmp wins over the text export",
			geoTmp:     syntheticGeoTmp(),
			hoehenText: hoehen,
			wantPoints: 1,
			wantLines:  1,
		},
		{
			name:       "the text export is used when GeoTmp is absent",
			hoehenText: hoehen,
			wantPoints: 2,
			wantLines:  0,
		},
		{
			name:       "a GeoTmp with no features falls back to the text export",
			geoTmp:     geoObjectRecord(wireTypeElevPoint, 0),
			hoehenText: hoehen,
			wantPoints: 2,
			wantLines:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if tc.geoTmp != nil {
				writeFixture(t, dir, "GeoTmp.geo", tc.geoTmp)
			}

			writeFixture(t, dir, "Höhen.txt", []byte(tc.hoehenText))

			terrain, err := LoadTerrainData(dir)
			if err != nil {
				t.Fatalf("LoadTerrainData: %v", err)
			}

			if len(terrain.ElevationPoints) != tc.wantPoints {
				t.Errorf("elevation points = %d, want %d", len(terrain.ElevationPoints), tc.wantPoints)
			}

			if len(terrain.ContourLines) != tc.wantLines {
				t.Errorf("contour lines = %d, want %d", len(terrain.ContourLines), tc.wantLines)
			}
		})
	}
}

// TestLoadTerrainData_DGMFiles pins the supplemental ground models: every
// *.dgm in the directory is loaded in sorted order, and a file that will not
// parse degrades to a warning once some terrain has already loaded.
func TestLoadTerrainData_DGMFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFixture(t, dir, "GeoTmp.geo", syntheticGeoTmp())
	writeFixture(t, dir, "RDGM0002.dgm", dgmImage([]ElevationPoint{{X: 2, Y: 2, Z: 2}}, -1))
	writeFixture(t, dir, "RDGM0001.dgm", dgmImage([]ElevationPoint{{X: 1, Y: 1, Z: 1}}, -1))
	writeFixture(t, dir, "RDGM0003.dgm", []byte("not a ground model"))

	terrain, err := LoadTerrainData(dir)
	if err != nil {
		t.Fatalf("LoadTerrainData: %v", err)
	}

	if len(terrain.DGMFiles) != 2 {
		t.Fatalf("got %d DGM files, want the two that parse", len(terrain.DGMFiles))
	}

	if terrain.DGMFiles[0].SourceFile != "RDGM0001.dgm" || terrain.DGMFiles[1].SourceFile != "RDGM0002.dgm" {
		t.Errorf("DGM order = %q, %q; want sorted by name",
			terrain.DGMFiles[0].SourceFile, terrain.DGMFiles[1].SourceFile)
	}

	if len(terrain.Warnings) != 1 || !strings.Contains(terrain.Warnings[0], "RDGM0003.dgm") {
		t.Errorf("warnings = %v, want one naming the unreadable ground model", terrain.Warnings)
	}
}

// TestLoadTerrainData_DGMOnly pins that a project with no base terrain but a
// readable ground model still loads: the DGM is terrain in its own right.
func TestLoadTerrainData_DGMOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFixture(t, dir, "RDGM0001.dgm", dgmImage([]ElevationPoint{{X: 1, Y: 1, Z: 1}}, -1))

	terrain, err := LoadTerrainData(dir)
	if err != nil {
		t.Fatalf("LoadTerrainData: %v", err)
	}

	if len(terrain.ElevationPoints) != 0 || len(terrain.DGMFiles) != 1 {
		t.Errorf("terrain = %+v, want only the ground model", terrain)
	}
}

// TestLoadTerrainData_NoTerrainAtAll pins the hard failure and, more
// importantly, that the error names every source that was tried. A bare "no
// terrain" would not say whether the text export was even looked for.
func TestLoadTerrainData_NoTerrainAtAll(t *testing.T) {
	t.Parallel()

	_, err := LoadTerrainData(t.TempDir())
	if err == nil {
		t.Fatal("got nil error for a directory with no terrain")
	}

	for _, want := range []string{"geotmp=", "hoehen="} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to mention %q", err, want)
		}
	}
}

// TestLoadTerrainData_UnreadableDGMIsFatalWithoutBaseTerrain pins the other
// side of the DGM degradation rule: with nothing else loaded, a ground model
// that will not parse is the failure rather than a warning on an empty result.
func TestLoadTerrainData_UnreadableDGMIsFatalWithoutBaseTerrain(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFixture(t, dir, "RDGM0001.dgm", []byte("not a ground model"))

	_, err := LoadTerrainData(dir)
	if err == nil {
		t.Fatal("got nil error, want the unreadable ground model to be fatal here")
	}

	if !strings.Contains(err.Error(), "dgm=RDGM0001.dgm") {
		t.Errorf("error = %q, want it to name the ground model", err)
	}
}
