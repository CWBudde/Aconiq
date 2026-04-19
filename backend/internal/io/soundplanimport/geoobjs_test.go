package soundplanimport

import (
	"math"
	"path/filepath"
	"testing"
)

func TestParseGeoObjs_BuildingCount(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	objs, err := ParseGeoObjsFile(filepath.Join(dir, "GeoObjs.geo"))
	if err != nil {
		t.Fatalf("ParseGeoObjsFile: %v", err)
	}

	// The sample project has 315 building polygons (type 0x03ec).
	if len(objs.Buildings) != 315 {
		t.Errorf("got %d buildings, want 315", len(objs.Buildings))
	}
}

func TestParseGeoObjs_BuildingsAreClosed(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	objs, err := ParseGeoObjsFile(filepath.Join(dir, "GeoObjs.geo"))
	if err != nil {
		t.Fatalf("ParseGeoObjsFile: %v", err)
	}

	for i, b := range objs.Buildings {
		if len(b.Footprint) < 4 {
			t.Errorf("building %d: only %d points, want >= 4 (closed polygon)", i, len(b.Footprint))

			continue
		}

		first := b.Footprint[0]
		last := b.Footprint[len(b.Footprint)-1]

		if math.Abs(first.X-last.X) > 0.01 || math.Abs(first.Y-last.Y) > 0.01 {
			t.Errorf("building %d: not closed, first=(%.2f,%.2f) last=(%.2f,%.2f)",
				i, first.X, first.Y, last.X, last.Y)
		}
	}
}

func TestParseGeoObjs_BuildingCoordinatesPlausible(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	objs, err := ParseGeoObjsFile(filepath.Join(dir, "GeoObjs.geo"))
	if err != nil {
		t.Fatalf("ParseGeoObjsFile: %v", err)
	}

	for i, b := range objs.Buildings {
		for j, pt := range b.Footprint {
			if pt.X < 6000 || pt.X > 9000 || pt.Y < 5000 || pt.Y > 8000 {
				t.Errorf("building %d pt %d: (%.2f,%.2f) out of expected range", i, j, pt.X, pt.Y)

				break
			}

			if pt.Z < 200 || pt.Z > 300 {
				t.Errorf("building %d pt %d: Z=%.2f out of expected range [200,300]", i, j, pt.Z)

				break
			}
		}
	}
}

func TestParseGeoObjs_ReceiverCount(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	objs, err := ParseGeoObjsFile(filepath.Join(dir, "GeoObjs.geo"))
	if err != nil {
		t.Fatalf("ParseGeoObjsFile: %v", err)
	}

	// The sample project has 77 receiver points (type 0x0028).
	if len(objs.Receivers) != 77 {
		t.Errorf("got %d receivers, want 77", len(objs.Receivers))
	}
}

func TestParseGeoObjs_ReceiverCoordinatesPlausible(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	objs, err := ParseGeoObjsFile(filepath.Join(dir, "GeoObjs.geo"))
	if err != nil {
		t.Fatalf("ParseGeoObjsFile: %v", err)
	}

	for i, r := range objs.Receivers {
		if r.X < 6000 || r.X > 9000 || r.Y < 5000 || r.Y > 8000 {
			t.Errorf("receiver %d: (%.2f,%.2f) out of expected range", i, r.X, r.Y)
		}
	}
}

func TestParseGeoObjs_BuildingHeightsExtracted(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	objs, err := ParseGeoObjsFile(filepath.Join(dir, "GeoObjs.geo"))
	if err != nil {
		t.Fatalf("ParseGeoObjsFile: %v", err)
	}

	for i, building := range objs.Buildings {
		if building.HeightM <= 0 {
			t.Fatalf("building %d: height_m = %.3f, want > 0", i, building.HeightM)
		}
	}

	wantHeights := []float64{6.1, 3.1, 3.1, 6.1, 16.1, 2.1}
	for i, want := range wantHeights {
		if math.Abs(objs.Buildings[i].HeightM-want) > 0.01 {
			t.Fatalf("building %d: height_m = %.3f, want %.1f", i, objs.Buildings[i].HeightM, want)
		}
	}
}

func TestParseGeoObjs_BuildingAddressesAssigned(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	objs, err := ParseGeoObjsFile(filepath.Join(dir, "GeoObjs.geo"))
	if err != nil {
		t.Fatalf("ParseGeoObjsFile: %v", err)
	}

	addresses := make(map[string]bool)
	buildingsWithAddress := 0
	for _, building := range objs.Buildings {
		if len(building.Addresses) == 0 {
			continue
		}

		buildingsWithAddress++
		for _, address := range building.Addresses {
			addresses[address] = true
		}
	}

	if buildingsWithAddress < 10 {
		t.Fatalf("got %d buildings with addresses, want at least 10", buildingsWithAddress)
	}

	for _, want := range []string{"Hauptstraße 4", "Wallchenstraße 25", "Grasmückenweg 11"} {
		if !addresses[want] {
			t.Fatalf("expected address %q in parsed building metadata", want)
		}
	}
}
