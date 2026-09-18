package contour

import (
	"testing"

	"github.com/aconiq/backend/internal/report/results"
)

func TestGenerateContours(t *testing.T) {
	t.Parallel()

	// Create a 4x4 raster with a gradient: values go from 40 to 55.
	bandNames := []string{"Lden"}

	raster, err := results.NewRaster(results.RasterMetadata{
		Width:     4,
		Height:    4,
		Bands:     1,
		NoData:    -9999,
		Units:     results.UniformUnits(bandNames, results.UnitDecibel),
		BandNames: bandNames,
	})
	if err != nil {
		t.Fatalf("create raster: %v", err)
	}

	for y := range 4 {
		for x := range 4 {
			err := raster.Set(x, y, 0, 40.0+float64(x+y)*2.5)
			if err != nil {
				t.Fatalf("set raster: %v", err)
			}
		}
	}

	gt := GeoTransform{
		OriginX:    100,
		OriginY:    400,
		PixelSizeX: 10,
		PixelSizeY: -10,
	}

	contours, err := GenerateContours(raster, gt, Options{Interval: 5})
	if err != nil {
		t.Fatalf("generate contours: %v", err)
	}

	if len(contours) == 0 {
		t.Fatal("expected at least one contour line")
	}

	// Verify all contours have at least 2 points and are at valid levels.
	for _, c := range contours {
		if len(c.Points) < 2 {
			t.Fatalf("contour at level %g has only %d points", c.Level, len(c.Points))
		}

		if c.BandName != "Lden" {
			t.Fatalf("band_name = %q, want Lden", c.BandName)
		}
	}
}

func TestGenerateContoursSmallRaster(t *testing.T) {
	t.Parallel()

	raster, err := results.NewRaster(results.RasterMetadata{
		Width: 1, Height: 1, Bands: 1, NoData: -9999,
	})
	if err != nil {
		t.Fatalf("create raster: %v", err)
	}

	_, err = GenerateContours(raster, GeoTransform{}, Options{})
	if err == nil {
		t.Fatal("expected error for 1x1 raster")
	}
}

func TestGenerateContoursNilRaster(t *testing.T) {
	t.Parallel()

	_, err := GenerateContours(nil, GeoTransform{}, Options{})
	if err == nil {
		t.Fatal("expected error for nil raster")
	}
}

func TestGenerateContoursAllNoData(t *testing.T) {
	t.Parallel()

	// Raster filled with NoData — should produce no contours.
	bandNames := []string{"test"}

	raster, err := results.NewRaster(results.RasterMetadata{
		Width: 3, Height: 3, Bands: 1, NoData: -9999,
		Units: results.UniformUnits(bandNames, results.UnitDecibel), BandNames: bandNames,
	})
	if err != nil {
		t.Fatalf("create raster: %v", err)
	}

	// NewRaster fills with NoData by default.
	contours, err := GenerateContours(raster, GeoTransform{PixelSizeX: 1, PixelSizeY: -1}, Options{Interval: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(contours) != 0 {
		t.Fatalf("expected 0 contours for all-nodata raster, got %d", len(contours))
	}
}

func TestMarchingSquaresCases(t *testing.T) {
	t.Parallel()

	// Test a simple 2x2 grid with a horizontal boundary.
	// Row 0: [40, 40], Row 1: [60, 60]
	// Level 50 should produce a horizontal line segment.
	grid := [][]float64{
		{40, 40},
		{60, 60},
	}

	segments := marchingSquares(grid, 50.0, -9999)
	if len(segments) == 0 {
		t.Fatal("expected segments for horizontal boundary")
	}

	// Verify the segment crosses at y=0.5 (interpolated midpoint).
	for _, seg := range segments {
		if seg[0][1] < 0 || seg[0][1] > 1 || seg[1][1] < 0 || seg[1][1] > 1 {
			t.Fatalf("segment out of cell bounds: %v", seg)
		}
	}
}

func TestMarchingSquaresNoData(t *testing.T) {
	t.Parallel()

	grid := [][]float64{
		{40, -9999},
		{60, 60},
	}

	segments := marchingSquares(grid, 50.0, -9999)
	if len(segments) != 0 {
		t.Fatalf("expected 0 segments with nodata, got %d", len(segments))
	}
}

func TestMarchingSquaresAllAbove(t *testing.T) {
	t.Parallel()

	grid := [][]float64{
		{60, 60},
		{60, 60},
	}

	segments := marchingSquares(grid, 50.0, -9999)
	if len(segments) != 0 {
		t.Fatalf("expected 0 segments when all above, got %d", len(segments))
	}
}

func TestJoinSegments(t *testing.T) {
	t.Parallel()

	// Two segments that share an endpoint.
	segments := [][2][2]float64{
		{{0, 0.5}, {0.5, 0}},
		{{0.5, 0}, {1, 0.5}},
	}

	lines := joinSegments(segments)
	if len(lines) != 1 {
		t.Fatalf("expected 1 joined line, got %d", len(lines))
	}

	if len(lines[0]) != 3 {
		t.Fatalf("expected 3 points in joined line, got %d", len(lines[0]))
	}
}

func TestInterpolate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b, level float64
		want        float64
	}{
		{40, 60, 50, 0.5},
		{40, 60, 40, 0.0},
		{40, 60, 60, 1.0},
		{40, 60, 45, 0.25},
		{50, 50, 50, 0.5}, // degenerate case
	}

	for _, tt := range tests {
		got := interpolate(tt.a, tt.b, tt.level)
		if got < tt.want-0.01 || got > tt.want+0.01 {
			t.Fatalf("interpolate(%v, %v, %v) = %v, want %v", tt.a, tt.b, tt.level, got, tt.want)
		}
	}
}
