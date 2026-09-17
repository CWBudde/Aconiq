package soundplanimport

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

// Fixture-free tests for the remaining *.geo scanners and for the .dgm ground
// model: GeoWand, GeoTmp, CalcArea, GeoRail and RDGM. As in
// geoobjs_synthetic_test.go every byte is authored from the layout the parser
// itself declares, so all of this runs without the licensed project.

// railObjectRecord, railNameRecord and railParamsRecord build the three
// GeoRail.geo records that carry information. The object header is the same
// 44-byte :O& record the other scanners see; parseGeoRailData only uses its
// marker, and walks the rest of it byte by byte.
func railNameRecord(name []byte, declaredLen int) []byte {
	length := len(name)
	if declaredLen >= 0 {
		length = declaredLen
	}

	rec := make([]byte, 15, 15+len(name))
	rec[0] = ':'
	rec[1] = 'D'
	rec[2] = '1'
	rec[14] = byte(length)

	return append(rec, name...)
}

// railParamsRecord writes the 20 float64s handleParams reads, of which it
// decodes the speed (0), the bridge correction (9) and the track height (18).
func railParamsRecord(speed float64, bridge float64, trackHeight float64) []byte {
	rec := make([]byte, 15+20*8)
	rec[0] = ':'
	rec[1] = 'D'
	rec[2] = '='

	copy(rec[15:], leF64(speed))
	copy(rec[15+9*8:], leF64(bridge))
	copy(rec[15+18*8:], leF64(trackHeight))

	return rec
}

// TestParseGeoWandData_Synthetic pins the barrier decode: geometry, the :D!
// acoustic record, and the sign of a material code.
func TestParseGeoWandData_Synthetic(t *testing.T) {
	t.Parallel()

	wall := concat(
		geoObjectRecord(wireTypeWall, 0),
		geoDataRecord('!', barrierAcousticsPayload(4, 0, 12), -1),
		geoPointRecord(10, 10, 215, 4),
		geoPointRecord(30, 10, 215.5, 4.5),
		geoPointRecord(50, 10, 216, 5),
	)

	// SoundPLAN writes an all-ones material code for "not set"; readI64 must
	// reinterpret it as -1 so the consumer's MaterialCode >= 0 guard sees it.
	unsetMaterial := concat(
		geoObjectRecord(wireTypeWall, 0),
		geoDataRecord('!', barrierAcousticsPayload(0, 0, math.MaxUint64), -1),
		geoPointRecord(70, 10, 220, 3),
	)

	barriers := parseGeoWandData(concat(wall, unsetMaterial))

	if len(barriers) != 2 {
		t.Fatalf("got %d barriers, want 2", len(barriers))
	}

	first := barriers[0]
	if len(first.Points) != 3 {
		t.Fatalf("first barrier has %d points, want 3", len(first.Points))
	}

	if first.Points[1] != (BarrierPoint{X: 30, Y: 10, ZTop: 215.5, Height: 4.5}) {
		t.Errorf("points[1] = %+v", first.Points[1])
	}

	if !first.HasAcousticProperties || first.AbsorptionSideADB != 4 || first.AbsorptionSideBDB != 0 {
		t.Errorf("acoustics = %+v", first)
	}

	if first.MaterialCode != 12 {
		t.Errorf("MaterialCode = %d, want 12", first.MaterialCode)
	}

	if barriers[1].MaterialCode != -1 {
		t.Errorf("unset MaterialCode = %d, want -1", barriers[1].MaterialCode)
	}
}

// TestParseGeoWandData_MalformedRecords pins the refusals: points that do not
// belong to a wall object, an acoustic record too short for its layout, and a
// data record whose declared length runs past the file.
func TestParseGeoWandData_MalformedRecords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		image []byte
		check func(t *testing.T, barriers []NoiseBarrier)
	}{
		{
			name: "points of a non-wall object are dropped",
			image: concat(
				geoObjectRecord(wireTypeBuilding, 0),
				geoPointRecord(1, 2, 3, 4),
				geoPointRecord(5, 6, 7, 8),
			),
			check: func(t *testing.T, barriers []NoiseBarrier) {
				t.Helper()

				if len(barriers) != 0 {
					t.Errorf("got %d barriers from a building object, want none", len(barriers))
				}
			},
		},
		{
			name: "an acoustic payload shorter than the layout leaves the flag unset",
			image: concat(
				geoObjectRecord(wireTypeWall, 0),
				geoDataRecord('!', barrierAcousticsPayload(4, 2, 7)[:23], -1),
				geoPointRecord(1, 2, 3, 4),
			),
			check: func(t *testing.T, barriers []NoiseBarrier) {
				t.Helper()

				if len(barriers) != 1 {
					t.Fatalf("got %d barriers, want 1", len(barriers))
				}

				if barriers[0].HasAcousticProperties {
					t.Error("HasAcousticProperties was set from a payload too short to hold the values")
				}

				if len(barriers[0].Points) != 1 {
					t.Errorf("points = %+v, want the point after the short record", barriers[0].Points)
				}
			},
		},
		{
			name: "a data record declaring a payload past the end of the file is not read",
			image: concat(
				geoObjectRecord(wireTypeWall, 0),
				geoDataRecord('!', barrierAcousticsPayload(4, 2, 7), math.MaxUint32),
				geoPointRecord(1, 2, 3, 4),
			),
			check: func(t *testing.T, barriers []NoiseBarrier) {
				t.Helper()

				if len(barriers) != 1 || barriers[0].HasAcousticProperties {
					t.Errorf("barriers = %+v, want one barrier with no acoustic properties", barriers)
				}
			},
		},
		{
			name: "an unrelated data record is skipped without losing the points",
			image: concat(
				geoObjectRecord(wireTypeWall, 0),
				geoDataRecord('1', pascalStringPayload([]byte("Wand"), -1), -1),
				geoPointRecord(1, 2, 3, 4),
			),
			check: func(t *testing.T, barriers []NoiseBarrier) {
				t.Helper()

				if len(barriers) != 1 || len(barriers[0].Points) != 1 {
					t.Errorf("barriers = %+v, want one barrier with one point", barriers)
				}
			},
		},
		{
			name:  "a wall object with no points produces no barrier",
			image: concat(geoObjectRecord(wireTypeWall, 0), geoObjectRecord(wireTypeWall, 0), geoPointRecord(1, 2, 3, 4)),
			check: func(t *testing.T, barriers []NoiseBarrier) {
				t.Helper()

				if len(barriers) != 1 {
					t.Errorf("got %d barriers, want 1: an empty object group is not a barrier", len(barriers))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tc.check(t, parseGeoWandData(tc.image))
		})
	}
}

// TestParseGeoTmpData_Synthetic pins how the terrain layer is split: object
// type 0x040b is a single elevation sample, 0x040a and 0x046e are polylines,
// and anything else contributes nothing.
func TestParseGeoTmpData_Synthetic(t *testing.T) {
	t.Parallel()

	image := concat(
		geoObjectRecord(wireTypeElevPoint, 0),
		geoPointRecord(100, 200, 210.5, 0),
		geoObjectRecord(wireTypeContour, 0),
		geoPointRecord(0, 0, 200, 0),
		geoPointRecord(10, 0, 200, 0),
		geoPointRecord(20, 0, 200, 0),
		geoObjectRecord(wireTypeTerrainBreak, 0),
		geoPointRecord(0, 50, 205, 0),
		geoPointRecord(10, 50, 206, 0),
		geoObjectRecord(0x1234, 0),
		geoPointRecord(999, 999, 999, 0),
		geoObjectRecord(wireTypeElevPoint, 0),
		geoPointRecord(300, 400, 212, 0),
	)

	terrain := parseGeoTmpData(image)

	if len(terrain.ElevationPoints) != 2 {
		t.Fatalf("elevation points = %+v, want 2", terrain.ElevationPoints)
	}

	if terrain.ElevationPoints[0] != (ElevationPoint{X: 100, Y: 200, Z: 210.5}) {
		t.Errorf("elevation point 0 = %+v", terrain.ElevationPoints[0])
	}

	if len(terrain.ContourLines) != 2 {
		t.Fatalf("contour lines = %d, want 2 (a contour and a break line)", len(terrain.ContourLines))
	}

	if len(terrain.ContourLines[0].Points) != 3 || len(terrain.ContourLines[1].Points) != 2 {
		t.Errorf("contour point counts = %d/%d, want 3/2",
			len(terrain.ContourLines[0].Points), len(terrain.ContourLines[1].Points))
	}

	// A group of an unrecognised type must not leak into either collection.
	for _, point := range terrain.ElevationPoints {
		if point.X == 999 {
			t.Error("a point from an unknown object type reached the elevation samples")
		}
	}
}

// TestParseGeoTmpData_TruncatedPointIsDropped pins that a partial :G record at
// the end of the file contributes nothing rather than a point built from
// whatever bytes happen to follow.
func TestParseGeoTmpData_TruncatedPointIsDropped(t *testing.T) {
	t.Parallel()

	full := geoPointRecord(1, 2, 3, 0)

	terrain := parseGeoTmpData(concat(
		geoObjectRecord(wireTypeContour, 0),
		full,
		full,
		full[:30],
	))

	if len(terrain.ContourLines) != 1 || len(terrain.ContourLines[0].Points) != 2 {
		t.Fatalf("contour = %+v, want one line with the two complete points", terrain.ContourLines)
	}
}

// TestParseCalcAreaData_Synthetic pins the calculation-area scan, which reads
// :G records wherever they appear and does not group them by object.
func TestParseCalcAreaData_Synthetic(t *testing.T) {
	t.Parallel()

	area, err := parseCalcAreaData(concat(
		geoObjectRecord(0x03ee, 0),
		geoPointRecord(0, 0, 0, 0),
		geoPointRecord(100, 0, 0, 0),
		geoPointRecord(100, 80, 0, 0),
		geoPointRecord(0, 80, 0, 0),
		geoPointRecord(0, 0, 0, 0),
	))
	if err != nil {
		t.Fatalf("parseCalcAreaData: %v", err)
	}

	if len(area.Points) != 5 {
		t.Fatalf("got %d points, want 5", len(area.Points))
	}

	if area.Points[2] != (Point3D{X: 100, Y: 80}) {
		t.Errorf("points[2] = %+v, want (100,80,0)", area.Points[2])
	}

	if area.Points[0] != area.Points[4] {
		t.Errorf("polygon is not closed: %+v vs %+v", area.Points[0], area.Points[4])
	}
}

// TestParseCalcAreaData_Refusals pins the two inputs the calculation-area
// parser must reject rather than answer with a degenerate polygon.
func TestParseCalcAreaData_Refusals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		image []byte
	}{
		{name: "empty file", image: nil},
		{name: "no point records", image: geoObjectRecord(0x03ee, 0)},
		{name: "only a truncated point record", image: geoPointRecord(1, 2, 3, 0)[:30]},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseCalcAreaData(tc.image)
			if err == nil {
				t.Fatal("got nil error, want a refusal")
			}

			if !strings.Contains(err.Error(), "no points found") {
				t.Errorf("error = %q, want it to say that no points were found", err)
			}
		})
	}
}

// TestParseGeoRailData_Synthetic pins the rail decode: the track name, the
// point quadruple, and the three parameters read out of the :D= record.
//
// It also pins the segmenting contract, which is easy to get backwards: a :D=
// record closes the run of points that precedes it, so the parameters of a
// segment are the ones written *after* its points. A file that declared its
// parameters first would attribute each segment the next segment's values.
func TestParseGeoRailData_Synthetic(t *testing.T) {
	t.Parallel()

	image := concat(
		geoObjectRecord(0, 0),
		railNameRecord([]byte("Gleis 1"), -1),
		geoPointRecord(1000, 2000, 210, 209),
		geoPointRecord(1100, 2000, 210.2, 209.1),
		railParamsRecord(160, -1000, 0.5),
		geoPointRecord(1200, 2000, 210.4, 209.2),
		geoPointRecord(1300, 2000, 210.6, 209.3),
		railParamsRecord(120, 3, 0.8),
		geoObjectRecord(0, 0),
		railNameRecord([]byte("Gleis 2"), -1),
		geoPointRecord(1000, 2010, 210, 209),
		railParamsRecord(100, -1000, 0.4),
	)

	tracks, err := parseGeoRailData(image)
	if err != nil {
		t.Fatalf("parseGeoRailData: %v", err)
	}

	if len(tracks) != 2 {
		t.Fatalf("got %d tracks, want 2", len(tracks))
	}

	if tracks[0].Name != "Gleis 1" || tracks[1].Name != "Gleis 2" {
		t.Errorf("track names = %q/%q", tracks[0].Name, tracks[1].Name)
	}

	if len(tracks[0].Segments) != 2 {
		t.Fatalf("track 0 has %d segments, want 2", len(tracks[0].Segments))
	}

	first := tracks[0].Segments[0]
	if len(first.Points) != 2 {
		t.Fatalf("segment 0 has %d points, want 2", len(first.Points))
	}

	if first.Points[0] != (TrackPoint{X: 1000, Y: 2000, ZTrack: 210, ZGround: 209}) {
		t.Errorf("segment 0 point 0 = %+v", first.Points[0])
	}

	wantFirst := RailSegmentParams{Speed: 160, BridgeCorrection: -1000, TrackHeight: 0.5}
	if first.Params != wantFirst {
		t.Errorf("segment 0 params = %+v, want %+v", first.Params, wantFirst)
	}

	wantSecond := RailSegmentParams{Speed: 120, BridgeCorrection: 3, TrackHeight: 0.8}
	if tracks[0].Segments[1].Params != wantSecond {
		t.Errorf("segment 1 params = %+v, want %+v", tracks[0].Segments[1].Params, wantSecond)
	}

	if len(tracks[1].Segments) != 1 || tracks[1].Segments[0].Params.Speed != 100 {
		t.Errorf("track 1 segments = %+v", tracks[1].Segments)
	}
}

// TestParseGeoRailData_MalformedRecords pins the rail scanner's refusals.
func TestParseGeoRailData_MalformedRecords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		image []byte
		check func(t *testing.T, tracks []RailTrack)
	}{
		{
			name:  "no object group means no tracks",
			image: concat(geoPointRecord(1, 2, 3, 4), geoPointRecord(5, 6, 7, 8)),
			check: func(t *testing.T, tracks []RailTrack) {
				t.Helper()

				if len(tracks) != 0 {
					t.Errorf("got %d tracks without an object group, want none", len(tracks))
				}
			},
		},
		{
			name:  "a track with no points has no segments",
			image: concat(geoObjectRecord(0, 0), railNameRecord([]byte("Leer"), -1)),
			check: func(t *testing.T, tracks []RailTrack) {
				t.Helper()

				if len(tracks) != 1 {
					t.Fatalf("got %d tracks, want 1", len(tracks))
				}

				if tracks[0].Name != "Leer" || len(tracks[0].Segments) != 0 {
					t.Errorf("track = %+v, want a named track with no segments", tracks[0])
				}
			},
		},
		{
			name:  "a truncated parameter record is skipped",
			image: concat(geoObjectRecord(0, 0), geoPointRecord(1, 2, 3, 4), railParamsRecord(160, -1000, 0.5)[:100]),
			check: func(t *testing.T, tracks []RailTrack) {
				t.Helper()

				if len(tracks) != 1 || len(tracks[0].Segments) != 1 {
					t.Fatalf("tracks = %+v, want one track with one segment", tracks)
				}

				if tracks[0].Segments[0].Params != (RailSegmentParams{}) {
					t.Errorf("params = %+v, want the zero value from a truncated record",
						tracks[0].Segments[0].Params)
				}
			},
		},
		{
			name: "a name whose declared length runs past the file is not read",
			image: concat(
				geoObjectRecord(0, 0),
				geoPointRecord(1, 2, 3, 4),
				railNameRecord([]byte("Gleis 1"), 200),
			),
			check: func(t *testing.T, tracks []RailTrack) {
				t.Helper()

				if len(tracks) != 1 {
					t.Fatalf("got %d tracks, want 1", len(tracks))
				}

				if tracks[0].Name != "" {
					t.Errorf("Name = %q, want empty: the declared name is not in the file", tracks[0].Name)
				}

				if len(tracks[0].Segments) != 1 {
					t.Errorf("segments = %+v, want the point to survive", tracks[0].Segments)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tracks, err := parseGeoRailData(tc.image)
			if err != nil {
				t.Fatalf("parseGeoRailData: %v", err)
			}

			tc.check(t, tracks)
		})
	}
}

// dgmImage builds an RDGM*.dgm image: eight u32 header values, padding out to
// the vertex block, and one 24-byte record per vertex (float64 X, float64 Y,
// float32 Z).
//
// declaredVertexCount overrides the count written into the header so that a
// test can declare more vertices than the file holds.
func dgmImage(points []ElevationPoint, declaredVertexCount int) []byte {
	count := len(points)
	if declaredVertexCount >= 0 {
		count = declaredVertexCount
	}

	image := make([]byte, wireDGMVertexBlockOffset+len(points)*wireDGMVertexRecordSize)
	copy(image[wireDGMVertexCountIndex*4:], leU32(uint32(count)))

	for i, point := range points {
		off := wireDGMVertexBlockOffset + i*wireDGMVertexRecordSize
		copy(image[off:], leF64(point.X))
		copy(image[off+8:], leF64(point.Y))
		copy(image[off+16:], leF32(float32(point.Z)))
	}

	return image
}

// TestParseDGMData_Synthetic pins the vertex table decode, including that Z is
// a float32 in a 24-byte record.
func TestParseDGMData_Synthetic(t *testing.T) {
	t.Parallel()

	points := []ElevationPoint{
		{X: 1000.5, Y: 2000.25, Z: 210.5},
		{X: 1010.5, Y: 2000.25, Z: 211.25},
		{X: 1020.5, Y: 2000.25, Z: 212},
	}

	dgm, err := parseDGMData("rdgm0001.dgm", dgmImage(points, -1))
	if err != nil {
		t.Fatalf("parseDGMData: %v", err)
	}

	if dgm.SourceFile != "rdgm0001.dgm" {
		t.Errorf("SourceFile = %q", dgm.SourceFile)
	}

	if dgm.HeaderValues[wireDGMVertexCountIndex] != 3 {
		t.Errorf("header vertex count = %d, want 3", dgm.HeaderValues[wireDGMVertexCountIndex])
	}

	if len(dgm.Points) != len(points) {
		t.Fatalf("got %d points, want %d", len(dgm.Points), len(points))
	}

	for i, want := range points {
		if dgm.Points[i] != want {
			t.Errorf("points[%d] = %+v, want %+v", i, dgm.Points[i], want)
		}
	}
}

// TestParseDGMData_Refusals pins the three header states the parser refuses.
//
// The last one matters most: the vertex count is an unvalidated file field
// that sizes a slice, so a header may not be allowed to ask for more vertices
// than the file can hold. The bound must be checked against the file length
// before anything is allocated.
func TestParseDGMData_Refusals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		image    []byte
		wantText string
	}{
		{
			name:     "shorter than the header and vertex block offset",
			image:    make([]byte, wireDGMVertexBlockOffset-1),
			wantText: "file too short",
		},
		{
			name:     "header declares no vertices",
			image:    dgmImage(nil, 0),
			wantText: "missing vertex count",
		},
		{
			name:     "header declares more vertices than the file holds",
			image:    dgmImage([]ElevationPoint{{X: 1, Y: 2, Z: 3}}, 4),
			wantText: "vertex table truncated",
		},
		{
			name:     "header declares the largest count a u32 can hold",
			image:    dgmImage(nil, math.MaxUint32),
			wantText: "vertex table truncated",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseDGMData("rdgm0001.dgm", tc.image)
			if err == nil {
				t.Fatal("got nil error, want a refusal")
			}

			if !strings.Contains(err.Error(), tc.wantText) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantText)
			}
		})
	}
}

// TestParseGeoRailData_NameIsCopiedVerbatim pins that a rail track name is
// taken from the file as raw bytes.
//
// Unlike GeoObjs.geo's :D'1' record, which readPascalString decodes from
// Windows-1252, handleName copies the bytes straight into a Go string. The
// byte 0xFC — "ü" in Windows-1252, and not valid UTF-8 on its own — therefore
// survives as itself rather than becoming a rune. This is the shipped
// behaviour, pinned so that a future charset fix is a deliberate change with a
// visible diff rather than a silent one.
func TestParseGeoRailData_NameIsCopiedVerbatim(t *testing.T) {
	t.Parallel()

	tracks, err := parseGeoRailData(concat(
		geoObjectRecord(0, 0),
		railNameRecord([]byte("G\xfcterzuggleis"), -1),
		geoPointRecord(1, 2, 3, 4),
	))
	if err != nil {
		t.Fatalf("parseGeoRailData: %v", err)
	}

	if len(tracks) != 1 {
		t.Fatalf("got %d tracks, want 1", len(tracks))
	}

	if tracks[0].Name != "G\xfcterzuggleis" {
		t.Errorf("Name = %q, want the bytes copied verbatim", tracks[0].Name)
	}

	if tracks[0].Name == "Güterzuggleis" {
		t.Error("the name was decoded from Windows-1252; update this test together with that change")
	}
}

// TestParseGeoFiles_MissingFileErrors pins that each file-level entry point
// reports a missing file rather than an empty result.
func TestParseGeoFiles_MissingFileErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	tests := []struct {
		name string
		call func(path string) error
	}{
		{name: "GeoWand", call: func(path string) error { _, err := ParseGeoWandFile(path); return err }},
		{name: "GeoTmp", call: func(path string) error { _, err := ParseGeoTmpFile(path); return err }},
		{name: "GeoRail", call: func(path string) error { _, err := ParseGeoRailFile(path); return err }},
		{name: "CalcArea", call: func(path string) error { _, err := ParseCalcAreaFile(path); return err }},
		{name: "DGM", call: func(path string) error { _, err := ParseDGMFile(path); return err }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.call(filepath.Join(dir, "missing-"+tc.name))
			if err == nil {
				t.Fatalf("%s: got nil error for a missing file", tc.name)
			}
		})
	}
}
