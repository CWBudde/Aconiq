package soundplanimport

import (
	"math"
	"path/filepath"
	"testing"
)

// Fixture-free tests for the GeoObjs.geo record scanner.
//
// The bytes come from geofixtures_synthetic_test.go, which authors them from
// the offsets geoobjs.go declares. Nothing here reads the licensed project, so
// these run on a clean checkout while the tests in geoobjs_test.go skip.

// hauptstrasseCP1252 is "Hauptstraße 4" in Windows-1252: 0xDF is the encoding's
// sharp s. Read as UTF-8 it is not valid text at all, which is what makes it a
// test of the decoder rather than of the byte copy.
var hauptstrasseCP1252 = []byte("Hauptstra\xdfe 4")

// syntheticGeoObjs builds a GeoObjs.geo image holding one building, one
// Immissionsort and one map label.
func syntheticGeoObjs() []byte {
	building := concat(
		geoObjectRecord(wireTypeBuilding, 0),
		geoDataRecord(wireTagBuildingAtt, buildingAttrPayload(6.1), -1),
		geoPointRecord(100, 100, 210, 204),
		geoPointRecord(120, 100, 210, 204),
		geoPointRecord(120, 130, 210, 204),
		geoPointRecord(100, 130, 210, 204),
		geoPointRecord(100, 100, 210, 204),
	)

	immission := concat(
		geoObjectRecord(wireTypeImmissionPoint, 101),
		geoDataRecord(wireTagName, pascalStringPayload(hauptstrasseCP1252, -1), -1),
		geoDataRecord(wireTagFloors, floorAttrPayload(2.5, 2.8, 3), -1),
		geoDataRecord(wireTagLimits, noiseLimitsPayload(59, 49), -1),
		geoPointRecord(110, 99, 205, 204.5),
	)

	label := concat(
		geoObjectRecord(wireTypeMapLabel, 0),
		geoDataRecord(wireTagLabelText, pascalStringPayload([]byte("Br\xfccke |"), -1), -1),
		geoPointRecord(140, 140, 0, 0),
	)

	return concat(building, immission, label)
}

// TestParseGeoObjsData_Synthetic pins the decode of all three object types the
// parser distinguishes, field by field. It is the fixture-free restatement of
// what TestParseGeoObjs_ImmissionPointsMatchRREC checks against the licensed
// project: that 0x03e9 is the Immissionsort and 0x0028 the annotation layer,
// and that the floor stack reproduces the per-floor elevations.
func TestParseGeoObjsData_Synthetic(t *testing.T) {
	t.Parallel()

	objs := parseGeoObjsData(syntheticGeoObjs())

	if len(objs.Buildings) != 1 || len(objs.ImmissionPoints) != 1 || len(objs.MapLabels) != 1 {
		t.Fatalf("got %d buildings, %d immission points, %d map labels; want 1/1/1",
			len(objs.Buildings), len(objs.ImmissionPoints), len(objs.MapLabels))
	}

	building := objs.Buildings[0]
	if len(building.Footprint) != 5 {
		t.Fatalf("footprint = %d points, want 5", len(building.Footprint))
	}

	if building.Footprint[0] != (Point3D{X: 100, Y: 100, Z: 210}) {
		t.Errorf("footprint[0] = %+v, want (100,100,210)", building.Footprint[0])
	}

	if building.Footprint[0] != building.Footprint[4] {
		t.Errorf("footprint is not closed: %+v vs %+v", building.Footprint[0], building.Footprint[4])
	}

	if math.Abs(building.HeightM-6.1) > 1e-9 {
		t.Errorf("HeightM = %v, want 6.1", building.HeightM)
	}

	if len(building.Addresses) != 1 || building.Addresses[0] != "Hauptstraße 4" {
		t.Errorf("Addresses = %v, want [Hauptstraße 4]", building.Addresses)
	}

	point := objs.ImmissionPoints[0]
	want := ImmissionPoint{
		ObjID:             101,
		Name:              "Hauptstraße 4",
		X:                 110,
		Y:                 99,
		FloorRefZ:         205,
		GroundHeightM:     204.5,
		FloorCount:        3,
		FirstFloorOffsetM: 2.5,
		FloorSpacingM:     2.8,
		LimitDayDB:        59,
		LimitNightDB:      49,
		HasFloorAttrs:     true,
	}

	if point != want {
		t.Errorf("immission point = %+v, want %+v", point, want)
	}

	// FloorZ stacks from the reference elevation; FloorHeightM is that
	// elevation expressed above the cached ground, which is what the model
	// schema's height_m means.
	for floor, wantZ := range map[int]float64{1: 207.5, 2: 210.3, 3: 213.1} {
		if math.Abs(point.FloorZ(floor)-wantZ) > 1e-9 {
			t.Errorf("FloorZ(%d) = %v, want %v", floor, point.FloorZ(floor), wantZ)
		}
	}

	if math.Abs(point.FloorHeightM(1)-3) > 1e-9 {
		t.Errorf("FloorHeightM(1) = %v, want 3", point.FloorHeightM(1))
	}

	label := objs.MapLabels[0]
	if label.X != 140 || label.Y != 140 || label.Text != "Brücke |" {
		t.Errorf("map label = %+v, want (140,140) Brücke |", label)
	}
}

// TestParseGeoObjsData_ObjectIDNeedsTheFullRecord pins objectIDAt's guard: the
// object id is only read when the :O& record is exactly objHeaderLen bytes and
// the next record's marker follows it. Anything else yields 0, so a caller
// falls back to positional identity instead of joining RREC rows on an id read
// out of unrelated bytes.
func TestParseGeoObjsData_ObjectIDNeedsTheFullRecord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		separator []byte
		wantObjID int64
	}{
		{name: "next record follows immediately", separator: nil, wantObjID: 101},
		{name: "a filler byte sits where the next marker should be", separator: []byte{0x00}, wantObjID: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			image := concat(
				geoObjectRecord(wireTypeImmissionPoint, 101),
				tc.separator,
				geoPointRecord(10, 20, 30, 25),
			)

			objs := parseGeoObjsData(image)
			if len(objs.ImmissionPoints) != 1 {
				t.Fatalf("got %d immission points, want 1", len(objs.ImmissionPoints))
			}

			if objs.ImmissionPoints[0].ObjID != tc.wantObjID {
				t.Errorf("ObjID = %d, want %d", objs.ImmissionPoints[0].ObjID, tc.wantObjID)
			}

			if objs.ImmissionPoints[0].X != 10 {
				t.Errorf("X = %v, want 10: the point must survive an unreadable id", objs.ImmissionPoints[0].X)
			}
		})
	}
}

// TestParseGeoObjsData_MalformedRecords pins what the scanner refuses. Every
// case is a file the parser must survive: a partial record at the end of a
// truncated download, a length field that overruns the buffer, and attribute
// payloads too short for the layout they claim.
func TestParseGeoObjsData_MalformedRecords(t *testing.T) {
	t.Parallel()

	fullPoint := geoPointRecord(10, 20, 30, 25)

	tests := []struct {
		name  string
		image []byte
		check func(t *testing.T, objs *GeoObjects)
	}{
		{
			name:  "object header truncated mid-record",
			image: geoObjectRecord(wireTypeBuilding, 0)[:20],
			check: func(t *testing.T, objs *GeoObjects) {
				t.Helper()

				if len(objs.Buildings) != 0 {
					t.Errorf("got %d buildings from a truncated header, want none", len(objs.Buildings))
				}
			},
		},
		{
			name: "point record truncated mid-payload",
			image: concat(
				geoObjectRecord(wireTypeBuilding, 0),
				fullPoint,
				fullPoint[:20],
			),
			check: func(t *testing.T, objs *GeoObjects) {
				t.Helper()

				if len(objs.Buildings) != 1 || len(objs.Buildings[0].Footprint) != 1 {
					t.Errorf("footprint = %+v, want only the complete point", objs.Buildings)
				}
			},
		},
		{
			name: "data record declares a payload past the end of the file",
			image: concat(
				geoObjectRecord(wireTypeImmissionPoint, 101),
				geoDataRecord(wireTagName, pascalStringPayload([]byte("Weg 1"), -1), 1<<20),
				fullPoint,
			),
			check: func(t *testing.T, objs *GeoObjects) {
				t.Helper()

				if len(objs.ImmissionPoints) != 1 {
					t.Fatalf("got %d immission points, want 1", len(objs.ImmissionPoints))
				}

				if objs.ImmissionPoints[0].Name != "" {
					t.Errorf("Name = %q, want empty: the record's payload is not in the file",
						objs.ImmissionPoints[0].Name)
				}
			},
		},
		{
			name: "data record declares the largest payload a u32 can hold",
			image: concat(
				geoObjectRecord(wireTypeImmissionPoint, 101),
				geoDataRecord(wireTagFloors, floorAttrPayload(1, 3, 2), math.MaxUint32),
				fullPoint,
			),
			check: func(t *testing.T, objs *GeoObjects) {
				t.Helper()

				if len(objs.ImmissionPoints) != 1 {
					t.Fatalf("got %d immission points, want 1", len(objs.ImmissionPoints))
				}

				if objs.ImmissionPoints[0].HasFloorAttrs {
					t.Error("floor attributes were accepted from a record whose payload is not present")
				}
			},
		},
		{
			name: "floor attributes shorter than the layout leave the stack unset",
			image: concat(
				geoObjectRecord(wireTypeImmissionPoint, 101),
				geoDataRecord(wireTagFloors, floorAttrPayload(1, 3, 2)[:wireFloorAttrLen-1], -1),
				fullPoint,
			),
			check: func(t *testing.T, objs *GeoObjects) {
				t.Helper()

				point := objs.ImmissionPoints[0]
				if point.HasFloorAttrs || point.FloorCount != 0 || point.FloorSpacingM != 0 {
					t.Errorf("point = %+v, want no floor stack from a short payload", point)
				}
			},
		},
		{
			name: "noise limits shorter than the layout stay zero",
			image: concat(
				geoObjectRecord(wireTypeImmissionPoint, 101),
				geoDataRecord(wireTagLimits, noiseLimitsPayload(59, 49)[:wireLimitsLen-1], -1),
				fullPoint,
			),
			check: func(t *testing.T, objs *GeoObjects) {
				t.Helper()

				point := objs.ImmissionPoints[0]
				if point.LimitDayDB != 0 || point.LimitNightDB != 0 {
					t.Errorf("limits = %v/%v, want 0/0 from a short payload", point.LimitDayDB, point.LimitNightDB)
				}
			},
		},
		{
			name: "building attributes shorter than the height offset leave the height unset",
			image: concat(
				geoObjectRecord(wireTypeBuilding, 0),
				geoDataRecord(wireTagBuildingAtt, buildingAttrPayload(6.1)[:wireBuildingHeightOff], -1),
				fullPoint,
			),
			check: func(t *testing.T, objs *GeoObjects) {
				t.Helper()

				if objs.Buildings[0].HeightM != 0 {
					t.Errorf("HeightM = %v, want 0 from a short payload", objs.Buildings[0].HeightM)
				}
			},
		},
		{
			name: "a name longer than its payload is clamped to what is there",
			image: concat(
				geoObjectRecord(wireTypeImmissionPoint, 101),
				geoDataRecord(wireTagName, pascalStringPayload([]byte("Weg"), 200), -1),
				fullPoint,
			),
			check: func(t *testing.T, objs *GeoObjects) {
				t.Helper()

				if objs.ImmissionPoints[0].Name != "Weg" {
					t.Errorf("Name = %q, want %q", objs.ImmissionPoints[0].Name, "Weg")
				}
			},
		},
		{
			name: "an unknown data tag is skipped without disturbing the group",
			image: concat(
				geoObjectRecord(wireTypeBuilding, 0),
				geoDataRecord('Z', make([]byte, 16), -1),
				fullPoint,
			),
			check: func(t *testing.T, objs *GeoObjects) {
				t.Helper()

				if len(objs.Buildings) != 1 || len(objs.Buildings[0].Footprint) != 1 {
					t.Errorf("buildings = %+v, want one building with one point", objs.Buildings)
				}
			},
		},
		{
			name: "an unknown object type contributes nothing",
			image: concat(
				geoObjectRecord(0x0999, 5),
				fullPoint,
			),
			check: func(t *testing.T, objs *GeoObjects) {
				t.Helper()

				if len(objs.Buildings)+len(objs.ImmissionPoints)+len(objs.MapLabels) != 0 {
					t.Errorf("objects = %+v, want nothing decoded for an unknown type", objs)
				}
			},
		},
		{
			name:  "empty input",
			image: nil,
			check: func(t *testing.T, objs *GeoObjects) {
				t.Helper()

				if len(objs.Buildings)+len(objs.ImmissionPoints)+len(objs.MapLabels) != 0 {
					t.Errorf("objects = %+v, want nothing", objs)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.check(t, parseGeoObjsData(tc.image))
		})
	}
}

// TestParseGeoObjsData_AddressAnchorPrefersTheContainingBuilding pins the rule
// findNearestBuilding encodes: an address anchor inside a footprint belongs to
// that footprint even when another footprint's edge is nearer.
func TestParseGeoObjsData_AddressAnchorPrefersTheContainingBuilding(t *testing.T) {
	t.Parallel()

	// The anchor at (50,50) sits well inside the large building, 40 m from its
	// nearest edge, while the small building's edge is 1 m away.
	large := concat(
		geoObjectRecord(wireTypeBuilding, 0),
		geoPointRecord(10, 10, 0, 0),
		geoPointRecord(90, 10, 0, 0),
		geoPointRecord(90, 90, 0, 0),
		geoPointRecord(10, 90, 0, 0),
		geoPointRecord(10, 10, 0, 0),
	)

	small := concat(
		geoObjectRecord(wireTypeBuilding, 0),
		geoPointRecord(51, 49, 0, 0),
		geoPointRecord(53, 49, 0, 0),
		geoPointRecord(53, 51, 0, 0),
		geoPointRecord(51, 51, 0, 0),
		geoPointRecord(51, 49, 0, 0),
	)

	anchor := concat(
		geoObjectRecord(wireTypeImmissionPoint, 101),
		geoDataRecord(wireTagName, pascalStringPayload([]byte("Feldweg 2"), -1), -1),
		geoPointRecord(50, 50, 0, 0),
	)

	// A second Immissionsort with the same address must not duplicate it.
	duplicate := concat(
		geoObjectRecord(wireTypeImmissionPoint, 102),
		geoDataRecord(wireTagName, pascalStringPayload([]byte("Feldweg 2"), -1), -1),
		geoPointRecord(49, 51, 0, 0),
	)

	objs := parseGeoObjsData(concat(large, small, anchor, duplicate))

	if len(objs.Buildings) != 2 {
		t.Fatalf("got %d buildings, want 2", len(objs.Buildings))
	}

	if len(objs.Buildings[0].Addresses) != 1 || objs.Buildings[0].Addresses[0] != "Feldweg 2" {
		t.Errorf("large building addresses = %v, want exactly [Feldweg 2]", objs.Buildings[0].Addresses)
	}

	if len(objs.Buildings[1].Addresses) != 0 {
		t.Errorf("small building addresses = %v, want none: the anchor is not inside it",
			objs.Buildings[1].Addresses)
	}
}

// TestParseGeoObjsData_AnchorOutsideEveryBuildingTakesTheNearest pins the
// fallback branch: with no footprint containing the anchor, the nearest edge
// wins.
func TestParseGeoObjsData_AnchorOutsideEveryBuildingTakesTheNearest(t *testing.T) {
	t.Parallel()

	far := concat(
		geoObjectRecord(wireTypeBuilding, 0),
		geoPointRecord(500, 500, 0, 0),
		geoPointRecord(510, 500, 0, 0),
		geoPointRecord(510, 510, 0, 0),
		geoPointRecord(500, 500, 0, 0),
	)

	near := concat(
		geoObjectRecord(wireTypeBuilding, 0),
		geoPointRecord(0, 0, 0, 0),
		geoPointRecord(10, 0, 0, 0),
		geoPointRecord(10, 10, 0, 0),
		geoPointRecord(0, 0, 0, 0),
	)

	anchor := concat(
		geoObjectRecord(wireTypeImmissionPoint, 101),
		geoDataRecord(wireTagName, pascalStringPayload([]byte("Am Rand 1"), -1), -1),
		geoPointRecord(-5, 0, 0, 0),
	)

	objs := parseGeoObjsData(concat(far, near, anchor))

	if len(objs.Buildings[0].Addresses) != 0 {
		t.Errorf("far building took the address: %v", objs.Buildings[0].Addresses)
	}

	if len(objs.Buildings[1].Addresses) != 1 {
		t.Errorf("near building addresses = %v, want [Am Rand 1]", objs.Buildings[1].Addresses)
	}
}

// TestParseGeoObjsFile_MissingFile pins that an absent GeoObjs.geo is an error
// rather than an empty object set, which is what lets LoadProjectBundle tell
// "no geometry file" from "a geometry file with nothing in it".
func TestParseGeoObjsFile_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := ParseGeoObjsFile(filepath.Join(t.TempDir(), "GeoObjs.geo"))
	if err == nil {
		t.Fatal("ParseGeoObjsFile on a missing file: got nil error")
	}
}
