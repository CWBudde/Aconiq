package soundplanimport

import (
	"math"
	"path/filepath"
	"testing"
)

func TestParseGeoWand_BarrierCount(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	barriers, err := ParseGeoWandFile(filepath.Join(dir, "GeoWand.geo"))
	if err != nil {
		t.Fatalf("ParseGeoWandFile: %v", err)
	}

	// The sample project has one noise barrier wall.
	if len(barriers) != 1 {
		t.Fatalf("got %d barriers, want 1", len(barriers))
	}
}

func TestParseGeoWand_PointCount(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	barriers, err := ParseGeoWandFile(filepath.Join(dir, "GeoWand.geo"))
	if err != nil {
		t.Fatalf("ParseGeoWandFile: %v", err)
	}

	// The barrier has 11 coordinate points (from hex analysis, excluding the
	// embedded BMP thumbnail region which produces the 12th :G  false hit
	// in the raw scan — the parser's :O& scoping filters correctly).
	nPts := len(barriers[0].Points)
	if nPts < 10 || nPts > 15 {
		t.Errorf("barrier has %d points, want 10-15", nPts)
	}
}

func TestParseGeoWand_CoordinatesPlausible(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	barriers, err := ParseGeoWandFile(filepath.Join(dir, "GeoWand.geo"))
	if err != nil {
		t.Fatalf("ParseGeoWandFile: %v", err)
	}

	for i, pt := range barriers[0].Points {
		if pt.X < 6000 || pt.X > 9000 || pt.Y < 5000 || pt.Y > 8000 {
			t.Errorf("point %d: (%.2f,%.2f) out of expected coordinate range", i, pt.X, pt.Y)
		}

		if pt.ZTop < 200 || pt.ZTop > 300 {
			t.Errorf("point %d: ZTop=%.2f out of expected range [200,300]", i, pt.ZTop)
		}
	}
}

func TestParseGeoWand_HeightValues(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	barriers, err := ParseGeoWandFile(filepath.Join(dir, "GeoWand.geo"))
	if err != nil {
		t.Fatalf("ParseGeoWandFile: %v", err)
	}

	// The barrier has two height sections: 2.0m and 2.5m.
	has2m := false
	has2p5m := false

	for _, pt := range barriers[0].Points {
		if math.Abs(pt.Height-2.0) < 0.01 {
			has2m = true
		}

		if math.Abs(pt.Height-2.5) < 0.01 {
			has2p5m = true
		}
	}

	if !has2m {
		t.Error("no point with Height=2.0m found")
	}

	if !has2p5m {
		t.Error("no point with Height=2.5m found")
	}
}

func TestParseGeoWand_FirstPoint(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	barriers, err := ParseGeoWandFile(filepath.Join(dir, "GeoWand.geo"))
	if err != nil {
		t.Fatalf("ParseGeoWandFile: %v", err)
	}

	pt := barriers[0].Points[0]

	if math.Abs(pt.X-7701.07) > 0.01 {
		t.Errorf("first point X=%.2f, want ~7701.07", pt.X)
	}

	if math.Abs(pt.Y-6769.40) > 0.01 {
		t.Errorf("first point Y=%.2f, want ~6769.40", pt.Y)
	}

	if math.Abs(pt.ZTop-245.77) > 0.01 {
		t.Errorf("first point ZTop=%.2f, want ~245.77", pt.ZTop)
	}

	if math.Abs(pt.Height-2.0) > 0.01 {
		t.Errorf("first point Height=%.2f, want 2.0", pt.Height)
	}
}
