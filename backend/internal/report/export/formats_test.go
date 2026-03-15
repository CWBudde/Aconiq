package export

import (
	"testing"
)

func TestParseFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []Format
		wantErr bool
	}{
		{name: "empty", input: "", want: nil, wantErr: false},
		{name: "single geotiff", input: "geotiff", want: []Format{FormatGeoTIFF}, wantErr: false},
		{name: "multiple formats", input: "geotiff,gpkg,contour-geojson", want: []Format{FormatGeoTIFF, FormatGeoPackage, FormatContourGeoJSON}, wantErr: false},
		{name: "all formats", input: "geotiff,gpkg,contour-geojson,contour-gpkg", want: []Format{FormatGeoTIFF, FormatGeoPackage, FormatContourGeoJSON, FormatContourGeoPackage}, wantErr: false},
		{name: "with spaces", input: " geotiff , gpkg ", want: []Format{FormatGeoTIFF, FormatGeoPackage}, wantErr: false},
		{name: "case insensitive", input: "GeoTIFF,GPKG", want: []Format{FormatGeoTIFF, FormatGeoPackage}, wantErr: false},
		{name: "deduplicate", input: "geotiff,geotiff,gpkg", want: []Format{FormatGeoTIFF, FormatGeoPackage}, wantErr: false},
		{name: "unknown format", input: "geotiff,shapefile", wantErr: true},
		{name: "only commas", input: ",,,", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseFormats(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseFormats(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if tt.want == nil && got != nil {
				t.Fatalf("expected nil, got %v", got)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("ParseFormats(%q) = %v, want %v", tt.input, got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("ParseFormats(%q)[%d] = %v, want %v", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFormatMatrix(t *testing.T) {
	t.Parallel()

	matrix := FormatMatrix()
	if len(matrix) != len(AllFormats) {
		t.Fatalf("format matrix has %d entries, want %d", len(matrix), len(AllFormats))
	}

	for _, info := range matrix {
		if info.Format == "" || info.Label == "" || info.Category == "" || info.Extension == "" {
			t.Fatalf("incomplete format info: %+v", info)
		}
	}
}

func TestInferGeoTransformFromReceivers(t *testing.T) {
	t.Parallel()

	// 3x2 grid: X goes 10, 20, 30; Y goes 100, 200
	xs := []float64{10, 20, 30, 10, 20, 30}
	ys := []float64{100, 100, 100, 200, 200, 200}

	gt, err := InferGeoTransformFromReceivers(xs, ys, 3, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gt.PixelSizeX != 10.0 {
		t.Fatalf("PixelSizeX = %v, want 10", gt.PixelSizeX)
	}

	// PixelSizeY should be negative (south-pointing).
	if gt.PixelSizeY != -100.0 {
		t.Fatalf("PixelSizeY = %v, want -100", gt.PixelSizeY)
	}

	// Origin should be at top-left corner (half pixel offset).
	expectedOriginX := 10.0 - 10.0/2   // 5.0
	expectedOriginY := 200.0 + 100.0/2 // 250.0

	if gt.OriginX != expectedOriginX {
		t.Fatalf("OriginX = %v, want %v", gt.OriginX, expectedOriginX)
	}

	if gt.OriginY != expectedOriginY {
		t.Fatalf("OriginY = %v, want %v", gt.OriginY, expectedOriginY)
	}
}

func TestInferGeoTransformErrors(t *testing.T) {
	t.Parallel()

	_, err := InferGeoTransformFromReceivers(nil, nil, 0, 0)
	if err == nil {
		t.Fatal("expected error for empty coordinates")
	}

	_, err = InferGeoTransformFromReceivers([]float64{1}, []float64{2}, 3, 3)
	if err == nil {
		t.Fatal("expected error for mismatched count")
	}
}

func TestParseEPSGCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  int
	}{
		{"EPSG:25832", 25832},
		{"EPSG:4326", 4326},
		{"", 0},
		{"WKT:something", 0},
	}

	for _, tt := range tests {
		got := parseEPSGCode(tt.input)
		if got != tt.want {
			t.Fatalf("parseEPSGCode(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
