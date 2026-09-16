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

func TestParseGeoObjs_ImmissionPointCount(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	objs, err := ParseGeoObjsFile(filepath.Join(dir, "GeoObjs.geo"))
	if err != nil {
		t.Fatalf("ParseGeoObjsFile: %v", err)
	}

	// The sample project has 13 immission points (type 0x03e9). This test used
	// to assert 77 against type 0x0028, which is the map label layer.
	if len(objs.ImmissionPoints) != 13 {
		t.Errorf("got %d immission points, want 13", len(objs.ImmissionPoints))
	}
}

func TestParseGeoObjs_ImmissionPointCoordinatesPlausible(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	objs, err := ParseGeoObjsFile(filepath.Join(dir, "GeoObjs.geo"))
	if err != nil {
		t.Fatalf("ParseGeoObjsFile: %v", err)
	}

	for i, point := range objs.ImmissionPoints {
		if point.X < 6000 || point.X > 9000 || point.Y < 5000 || point.Y > 8000 {
			t.Errorf("immission point %d: (%.2f,%.2f) out of expected range", i, point.X, point.Y)
		}
	}
}

// TestParseGeoObjs_MapLabelCount pins what object type 0x0028 actually is.
//
// It is the drawing annotation layer: house numbers, and captions such as the
// bridge label, in which "|" is SoundPLAN's line break. Asserting the texts
// rather than only the count is what makes "these are not receivers" evidence
// instead of a comment.
func TestParseGeoObjs_MapLabelCount(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	objs, err := ParseGeoObjsFile(filepath.Join(dir, "GeoObjs.geo"))
	if err != nil {
		t.Fatalf("ParseGeoObjsFile: %v", err)
	}

	if len(objs.MapLabels) != 77 {
		t.Errorf("got %d map labels, want 77", len(objs.MapLabels))
	}

	texts := make(map[string]bool, len(objs.MapLabels))
	for _, label := range objs.MapLabels {
		texts[label.Text] = true
	}

	for _, want := range []string{"86", "Brücke |"} {
		if !texts[want] {
			t.Errorf("expected map label %q among the parsed captions", want)
		}
	}
}

// TestParseGeoObjs_ImmissionPointsMatchRREC is the test whose absence let the
// receiver import ship inverted for as long as it did.
//
// It joins the decoded immission points to the reference project's own
// receiver result table on the object id, and checks every row: position, name,
// floor count and the per-floor elevation the floor attributes reconstruct. If
// the two object types are ever swapped again, or the binary layout is read at
// the wrong offset, nothing here can pass.
func TestParseGeoObjs_ImmissionPointsMatchRREC(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	objs, err := ParseGeoObjsFile(filepath.Join(dir, "GeoObjs.geo"))
	if err != nil {
		t.Fatalf("ParseGeoObjsFile: %v", err)
	}

	rows, err := ParseReceiverResults(filepath.Join(dir, "RSPS0011", "RREC0011.abs"))
	if err != nil {
		t.Fatalf("ParseReceiverResults: %v", err)
	}

	if len(rows) != 30 {
		t.Fatalf("got %d RREC rows, want 30", len(rows))
	}

	byObjID := make(map[int64]ImmissionPoint, len(objs.ImmissionPoints))
	for _, point := range objs.ImmissionPoints {
		if _, exists := byObjID[point.ObjID]; exists {
			t.Fatalf("immission point object id %d is not unique", point.ObjID)
		}

		byObjID[point.ObjID] = point
	}

	rowsPerObjID := make(map[int32]int, len(byObjID))
	for _, row := range rows {
		rowsPerObjID[row.ObjID]++
	}

	const tolM = 1e-3

	for _, row := range rows {
		point, ok := byObjID[int64(row.ObjID)]
		if !ok {
			t.Fatalf("RREC row %d references object id %d, which no immission point carries", row.RecNo, row.ObjID)
		}

		if got := math.Hypot(point.X-row.X, point.Y-row.Y); got > tolM {
			t.Errorf("object %d: |ΔXY| = %g m, want <= %g", row.ObjID, got, tolM)
		}

		if point.Name != row.Name {
			t.Errorf("object %d: name = %q, want %q", row.ObjID, point.Name, row.Name)
		}

		if !point.HasFloorAttrs {
			t.Fatalf("object %d: floor attributes were not decoded", row.ObjID)
		}

		if point.FloorCount != rowsPerObjID[row.ObjID] {
			t.Errorf("object %d: floor count = %d, want %d result rows", row.ObjID, point.FloorCount, rowsPerObjID[row.ObjID])
		}

		if got := math.Abs(point.FloorZ(int(row.Floor)) - row.Z); got > tolM {
			t.Errorf("object %d floor %d: |ΔZ| = %g m, want <= %g", row.ObjID, row.Floor, got, tolM)
		}

		if point.LimitDayDB != row.Limit1 || point.LimitNightDB != row.Limit2 {
			t.Errorf("object %d: limits = %v/%v, want %v/%v", row.ObjID, point.LimitDayDB, point.LimitNightDB, row.Limit1, row.Limit2)
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
