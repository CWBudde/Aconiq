package fgbimport

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/gogama/flatgeobuf/flatgeobuf"
	"github.com/gogama/flatgeobuf/flatgeobuf/flat"
	flatbuffers "github.com/google/flatbuffers/go"
)

// --- Builders for the shapes buildFeature cannot produce ---

// partSpec is one nested geometry inside a multi-part feature.
type partSpec struct {
	geomType flat.GeometryType
	xy       []float64
	ends     []uint32
}

// buildPartedFeature creates a Feature whose geometry carries a parts vector,
// which is how FlatGeobuf encodes a MultiPolygon and how some writers encode a
// MultiLineString. buildFeature in fgbimport_test.go cannot express this shape.
func buildPartedFeature(geomType flat.GeometryType, parts []partSpec) flat.Feature {
	bldr := flatbuffers.NewBuilder(512)

	partOffsets := make([]flatbuffers.UOffsetT, len(parts))

	for i, part := range slices.Backward(parts) {
		flat.GeometryStartXyVector(bldr, len(part.xy))

		for _, v := range slices.Backward(part.xy) {
			bldr.PrependFloat64(v)
		}

		xyVec := bldr.EndVector(len(part.xy))

		var endsVec flatbuffers.UOffsetT

		if len(part.ends) > 0 {
			flat.GeometryStartEndsVector(bldr, len(part.ends))

			for _, v := range slices.Backward(part.ends) {
				bldr.PrependUint32(v)
			}

			endsVec = bldr.EndVector(len(part.ends))
		}

		flat.GeometryStart(bldr)
		flat.GeometryAddXy(bldr, xyVec)
		flat.GeometryAddType(bldr, part.geomType)

		if len(part.ends) > 0 {
			flat.GeometryAddEnds(bldr, endsVec)
		}

		partOffsets[i] = flat.GeometryEnd(bldr)
	}

	flat.GeometryStartPartsVector(bldr, len(parts))

	for _, v := range slices.Backward(partOffsets) {
		bldr.PrependUOffsetT(v)
	}

	partsVec := bldr.EndVector(len(parts))

	flat.GeometryStart(bldr)
	flat.GeometryAddType(bldr, geomType)
	flat.GeometryAddParts(bldr, partsVec)
	geomOff := flat.GeometryEnd(bldr)

	flat.FeatureStart(bldr)
	flat.FeatureAddGeometry(bldr, geomOff)
	featOff := flat.FeatureEnd(bldr)

	bldr.FinishSizePrefixed(featOff)

	return *flat.GetSizePrefixedRootAsFeature(bldr.FinishedBytes(), 0)
}

// buildHeaderCRS is buildHeader with an EPSG code declared in the header CRS.
func buildHeaderCRS(geomType flat.GeometryType, columns []testColumn, featureCount int, epsg int32) *flat.Header {
	bldr := flatbuffers.NewBuilder(256)

	colOffsets := make([]flatbuffers.UOffsetT, len(columns))
	for i, v := range slices.Backward(columns) {
		nameOff := bldr.CreateString(v.name)
		flat.ColumnStart(bldr)
		flat.ColumnAddName(bldr, nameOff)
		flat.ColumnAddType(bldr, v.colType)
		colOffsets[i] = flat.ColumnEnd(bldr)
	}

	flat.HeaderStartColumnsVector(bldr, len(columns))

	for _, v := range slices.Backward(colOffsets) {
		bldr.PrependUOffsetT(v)
	}

	colsVec := bldr.EndVector(len(columns))

	orgOff := bldr.CreateString("EPSG")

	flat.CrsStart(bldr)
	flat.CrsAddOrg(bldr, orgOff)
	flat.CrsAddCode(bldr, epsg)
	crsOff := flat.CrsEnd(bldr)

	flat.HeaderStart(bldr)
	flat.HeaderAddGeometryType(bldr, geomType)
	flat.HeaderAddColumns(bldr, colsVec)
	flat.HeaderAddIndexNodeSize(bldr, 0)
	flat.HeaderAddFeaturesCount(bldr, uint64(featureCount))
	flat.HeaderAddCrs(bldr, crsOff)
	hdrOff := flat.HeaderEnd(bldr)

	bldr.FinishSizePrefixed(hdrOff)

	return flat.GetSizePrefixedRootAsHeader(bldr.FinishedBytes(), 0)
}

// streamWithHeader assembles a FlatGeobuf stream around an explicit header.
func streamWithHeader(t *testing.T, hdr *flat.Header, features []flat.Feature) []byte {
	t.Helper()

	var buf bytes.Buffer

	w := flatgeobuf.NewFileWriter(&buf)

	_, err := w.Header(hdr)
	if err != nil {
		t.Fatalf("write header: %v", err)
	}

	if len(features) > 0 {
		_, err = w.Data(features)
		if err != nil {
			t.Fatalf("write features: %v", err)
		}
	}

	return buf.Bytes()
}

// coordsOf asserts that coords is a []any of the expected length.
func coordsOf(t *testing.T, coords any, want int, what string) []any {
	t.Helper()

	list, ok := coords.([]any)
	if !ok {
		t.Fatalf("%s: coordinates are %T, want []any", what, coords)
	}

	if len(list) != want {
		t.Fatalf("%s: got %d entries, want %d", what, len(list), want)
	}

	return list
}

// assertPoint asserts that value is an [x, y] pair.
func assertPoint(t *testing.T, value any, x, y float64, what string) {
	t.Helper()

	pair, ok := value.([]any)
	if !ok || len(pair) != 2 {
		t.Fatalf("%s: %#v is not an [x, y] pair", what, value)
	}

	if pair[0] != x || pair[1] != y {
		t.Fatalf("%s: got (%v, %v), want (%v, %v)", what, pair[0], pair[1], x, y)
	}
}

// --- Multi-part geometries ---

// A MultiPoint has neither ends nor parts: every coordinate pair is its own
// point. Nothing exercised this decoder before, so a version that emitted a
// single flat coordinate list — which is what LineString does with the same
// input — would have produced a GeoJSON MultiPoint that no consumer can read.
func TestGeometryToGeoJSONDecodesMultiPoint(t *testing.T) {
	t.Parallel()

	feat := buildFeature(flat.GeometryTypeMultiPoint, []float64{8.6, 47.3, 9.1, 48.0, 10.2, 49.4}, nil, nil)

	geomType, coords, err := geometryToGeoJSON(geometryOf(t, feat), flat.GeometryTypeMultiPoint)
	if err != nil {
		t.Fatalf("decode MultiPoint: %v", err)
	}

	if geomType != "MultiPoint" {
		t.Fatalf("geomType = %q, want MultiPoint", geomType)
	}

	points := coordsOf(t, coords, 3, "MultiPoint")
	assertPoint(t, points[0], 8.6, 47.3, "point 0")
	assertPoint(t, points[1], 9.1, 48.0, "point 1")
	assertPoint(t, points[2], 10.2, 49.4, "point 2")
}

// An empty MultiPoint yields an empty list rather than nil, so the GeoJSON it
// produces is still a valid MultiPoint.
func TestGeometryToGeoJSONDecodesEmptyMultiPoint(t *testing.T) {
	t.Parallel()

	feat := buildFeature(flat.GeometryTypeMultiPoint, nil, nil, nil)

	_, coords, err := geometryToGeoJSON(geometryOf(t, feat), flat.GeometryTypeMultiPoint)
	if err != nil {
		t.Fatalf("decode empty MultiPoint: %v", err)
	}

	points, ok := coords.([]any)
	if !ok {
		t.Fatalf("coordinates are %T, want []any", coords)
	}

	if points == nil {
		t.Fatal("an empty MultiPoint decoded to a nil coordinate list")
	}

	if len(points) != 0 {
		t.Fatalf("got %d points, want none", len(points))
	}
}

// A MultiPolygon carries one nested geometry per polygon, each with its own
// ends vector for its rings. Getting the nesting depth wrong here is the
// difference between a building footprint with a courtyard and one without.
func TestGeometryToGeoJSONDecodesMultiPolygonFromParts(t *testing.T) {
	t.Parallel()

	// Polygon 0: a unit square with a smaller inner ring (a courtyard).
	// Polygon 1: a single square offset to the right.
	feat := buildPartedFeature(flat.GeometryTypeMultiPolygon, []partSpec{
		{
			geomType: flat.GeometryTypePolygon,
			xy: []float64{
				0, 0, 4, 0, 4, 4, 0, 4, 0, 0,
				1, 1, 3, 1, 3, 3, 1, 3, 1, 1,
			},
			ends: []uint32{5, 10},
		},
		{
			geomType: flat.GeometryTypePolygon,
			xy:       []float64{10, 0, 12, 0, 12, 2, 10, 2, 10, 0},
			ends:     []uint32{5},
		},
	})

	geomType, coords, err := geometryToGeoJSON(geometryOf(t, feat), flat.GeometryTypeMultiPolygon)
	if err != nil {
		t.Fatalf("decode MultiPolygon: %v", err)
	}

	if geomType != "MultiPolygon" {
		t.Fatalf("geomType = %q, want MultiPolygon", geomType)
	}

	polygons := coordsOf(t, coords, 2, "MultiPolygon")

	outerAndInner := coordsOf(t, polygons[0], 2, "polygon 0")
	coordsOf(t, outerAndInner[0], 5, "polygon 0 outer ring")

	inner := coordsOf(t, outerAndInner[1], 5, "polygon 0 inner ring")
	assertPoint(t, inner[0], 1, 1, "polygon 0 inner ring point 0")

	second := coordsOf(t, polygons[1], 1, "polygon 1")

	ring := coordsOf(t, second[0], 5, "polygon 1 ring")
	assertPoint(t, ring[0], 10, 0, "polygon 1 ring point 0")
}

// A MultiPolygon holding exactly one polygon may be written without a parts
// vector at all, with the rings in the top-level ends vector. That fallback has
// to produce the same nesting a parted file would.
func TestGeometryToGeoJSONDecodesSinglePolygonMultiPolygon(t *testing.T) {
	t.Parallel()

	feat := buildFeature(
		flat.GeometryTypeMultiPolygon,
		[]float64{0, 0, 2, 0, 2, 2, 0, 2, 0, 0},
		[]uint32{5},
		nil,
	)

	_, coords, err := geometryToGeoJSON(geometryOf(t, feat), flat.GeometryTypeMultiPolygon)
	if err != nil {
		t.Fatalf("decode MultiPolygon without parts: %v", err)
	}

	polygons := coordsOf(t, coords, 1, "MultiPolygon")

	rings := coordsOf(t, polygons[0], 1, "polygon 0")
	coordsOf(t, rings[0], 5, "polygon 0 ring")
}

// A MultiLineString written with a parts vector nests one level deeper than one
// written with an ends vector, and both spellings occur in the wild.
func TestGeometryToGeoJSONDecodesMultiLineStringFromParts(t *testing.T) {
	t.Parallel()

	feat := buildPartedFeature(flat.GeometryTypeMultiLineString, []partSpec{
		{geomType: flat.GeometryTypeLineString, xy: []float64{0, 0, 1, 1}},
		{geomType: flat.GeometryTypeLineString, xy: []float64{5, 5, 6, 6, 7, 5}},
	})

	geomType, coords, err := geometryToGeoJSON(geometryOf(t, feat), flat.GeometryTypeMultiLineString)
	if err != nil {
		t.Fatalf("decode parted MultiLineString: %v", err)
	}

	if geomType != "MultiLineString" {
		t.Fatalf("geomType = %q, want MultiLineString", geomType)
	}

	lines := coordsOf(t, coords, 2, "MultiLineString")

	first := coordsOf(t, lines[0], 2, "line 0")
	assertPoint(t, first[0], 0, 0, "line 0 point 0")

	second := coordsOf(t, lines[1], 3, "line 1")
	assertPoint(t, second[2], 7, 5, "line 1 point 2")
}

// The ends-vector spelling of a MultiLineString must split at the declared
// offsets and nowhere else.
func TestGeometryToGeoJSONDecodesMultiLineStringFromEnds(t *testing.T) {
	t.Parallel()

	feat := buildFeature(
		flat.GeometryTypeMultiLineString,
		[]float64{0, 0, 1, 1, 5, 5, 6, 6, 7, 5},
		[]uint32{2, 5},
		nil,
	)

	_, coords, err := geometryToGeoJSON(geometryOf(t, feat), flat.GeometryTypeMultiLineString)
	if err != nil {
		t.Fatalf("decode MultiLineString: %v", err)
	}

	lines := coordsOf(t, coords, 2, "MultiLineString")
	coordsOf(t, lines[0], 2, "line 0")

	second := coordsOf(t, lines[1], 3, "line 1")
	assertPoint(t, second[0], 5, 5, "line 1 point 0")
}

// Neither parts nor ends means the whole coordinate vector is one line, wrapped
// so the result is still a MultiLineString.
func TestGeometryToGeoJSONDecodesSingleLineMultiLineString(t *testing.T) {
	t.Parallel()

	feat := buildFeature(flat.GeometryTypeMultiLineString, []float64{0, 0, 1, 1, 2, 0}, nil, nil)

	_, coords, err := geometryToGeoJSON(geometryOf(t, feat), flat.GeometryTypeMultiLineString)
	if err != nil {
		t.Fatalf("decode MultiLineString: %v", err)
	}

	lines := coordsOf(t, coords, 1, "MultiLineString")
	coordsOf(t, lines[0], 3, "line 0")
}

// A bad ends vector inside one part of a MultiPolygon must name the part, not
// just fail: a file with hundreds of buildings is unfixable otherwise.
func TestGeometryToGeoJSONReportsWhichPolygonPartIsBroken(t *testing.T) {
	t.Parallel()

	feat := buildPartedFeature(flat.GeometryTypeMultiPolygon, []partSpec{
		{geomType: flat.GeometryTypePolygon, xy: []float64{0, 0, 1, 0, 1, 1, 0, 0}, ends: []uint32{4}},
		{geomType: flat.GeometryTypePolygon, xy: []float64{0, 0, 1, 0, 1, 1, 0, 0}, ends: []uint32{0xFFFFFFFF}},
	})

	_, _, err := geometryToGeoJSON(geometryOf(t, feat), flat.GeometryTypeMultiPolygon)
	if err == nil {
		t.Fatal("expected an error for a ring end beyond the part's coordinates")
	}

	if !strings.Contains(err.Error(), "polygon 1") {
		t.Fatalf("error %q does not name the offending polygon", err)
	}
}

// The same for a MultiLineString read through its parts vector.
func TestGeometryToGeoJSONReportsWhichLinePartIsBroken(t *testing.T) {
	t.Parallel()

	feat := buildPartedFeature(flat.GeometryTypeMultiLineString, []partSpec{
		{geomType: flat.GeometryTypeLineString, xy: []float64{0, 0, 1, 1}},
		{geomType: flat.GeometryTypeLineString, xy: []float64{0, 0, 1, 1}},
	})

	// Corrupt the second part's xy vector length prefix so it claims more
	// coordinates than the buffer can hold.
	geom := geometryOf(t, feat)

	part := new(flat.Geometry)
	if !geom.Parts(part, 1) {
		t.Fatal("test geometry has no second part")
	}

	tab := part.Table()

	xyField := flatbuffers.UOffsetT(tab.Offset(6)) // Geometry.xy
	if xyField == 0 {
		t.Fatal("test part has no xy vector")
	}

	first := tab.Vector(xyField)
	putUint32(tab.Bytes[first-4:], 0xFFFFFFF0)

	_, _, err := geometryToGeoJSON(geom, flat.GeometryTypeMultiLineString)
	if err == nil {
		t.Fatal("expected an error for an xy vector longer than the buffer")
	}

	if !strings.Contains(err.Error(), "part 1") {
		t.Fatalf("error %q does not name the offending part", err)
	}
}

// A geometry that declares no type of its own inherits the header's, which is
// how a homogeneous FlatGeobuf file is written: only the header names the type.
func TestGeometryToGeoJSONFallsBackToTheHeaderGeometryType(t *testing.T) {
	t.Parallel()

	feat := buildFeature(flat.GeometryTypeUnknown, []float64{0, 0, 1, 1, 2, 0}, nil, nil)

	geomType, coords, err := geometryToGeoJSON(geometryOf(t, feat), flat.GeometryTypeLineString)
	if err != nil {
		t.Fatalf("decode with a header fallback: %v", err)
	}

	if geomType != "LineString" {
		t.Fatalf("geomType = %q, want LineString from the header", geomType)
	}

	coordsOf(t, coords, 3, "LineString")
}

// A geometry type this package does not map to GeoJSON must be named in the
// error rather than skipped, so a user importing a CircularString knows why
// nothing arrived.
func TestGeometryToGeoJSONRefusesUnsupportedGeometryTypes(t *testing.T) {
	t.Parallel()

	for _, geomType := range []flat.GeometryType{
		flat.GeometryTypeGeometryCollection,
		flat.GeometryTypeCircularString,
		flat.GeometryTypeCompoundCurve,
		flat.GeometryTypeCurvePolygon,
		flat.GeometryTypeTIN,
	} {
		feat := buildFeature(geomType, []float64{0, 0}, nil, nil)

		_, _, err := geometryToGeoJSON(geometryOf(t, feat), geomType)
		if err == nil {
			t.Fatalf("expected %s to be refused", geomType)
		}

		if !strings.Contains(err.Error(), "unsupported geometry type") {
			t.Fatalf("error %q does not say the type is unsupported", err)
		}
	}
}

// A Point whose xy vector holds fewer than two values has no coordinates. The
// decoder emits the feature anyway, with an empty coordinate list, and leaves
// the refusal to model normalization — this pins that it neither panics on the
// missing pair nor invents (0, 0), which would put a receiver on the equator.
func TestConvertFeatureEmitsAPointWithoutCoordinatesAsEmpty(t *testing.T) {
	t.Parallel()

	for _, xy := range [][]float64{nil, {8.6}} {
		feat := buildFeature(flat.GeometryTypePoint, xy, nil, nil)

		hdr := buildHeader(flat.GeometryTypePoint, nil, 1)

		result, ok, err := decodeFeature(&feat, hdr, flat.GeometryTypePoint, 0)
		if err != nil {
			t.Fatalf("decode %v: %v", xy, err)
		}

		if !ok {
			t.Fatalf("a Point with xy %v was dropped entirely", xy)
		}

		if result.Geometry.Type != "Point" {
			t.Fatalf("geometry type = %q, want Point", result.Geometry.Type)
		}

		coords, isList := result.Geometry.Coordinates.([]any)
		if !isList {
			t.Fatalf("coordinates are %T, want []any", result.Geometry.Coordinates)
		}

		if len(coords) != 0 {
			t.Fatalf("coordinates = %#v, want an empty list", coords)
		}
	}
}

// putUint32 writes a little-endian uint32, which is the byte order FlatBuffers
// uses for its vector length prefixes.
func putUint32(dst []byte, v uint32) {
	dst[0] = byte(v)
	dst[1] = byte(v >> 8)
	dst[2] = byte(v >> 16)
	dst[3] = byte(v >> 24)
}

// --- Header CRS ---

// The header CRS is what `aconiq import` uses to decide whether a file needs
// reprojecting, so it has to come through as an EPSG code rather than silently
// as 0 — which the importer reads as "unknown".
func TestReadWithCRSExtractsTheHeaderEPSGCode(t *testing.T) {
	t.Parallel()

	columns := []testColumn{{name: "kind", colType: flat.ColumnTypeString}}
	feat := buildFeature(flat.GeometryTypePoint, []float64{400000, 5600000}, nil, encodeProps(columns, []any{"building"}))

	stream := streamWithHeader(t, buildHeaderCRS(flat.GeometryTypePoint, columns, 1, 25832), []flat.Feature{feat})

	res, err := readAll(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}

	if res.EPSGCode != 25832 {
		t.Fatalf("EPSGCode = %d, want 25832", res.EPSGCode)
	}

	if len(res.Collection.Features) != 1 {
		t.Fatalf("read %d features, want 1", len(res.Collection.Features))
	}
}

// A header with no CRS, or one whose code is unset, reports 0 rather than
// inventing a code.
func TestReadWithCRSReportsZeroForAnUndeclaredCRS(t *testing.T) {
	t.Parallel()

	columns := []testColumn{{name: "kind", colType: flat.ColumnTypeString}}
	feat := buildFeature(flat.GeometryTypePoint, []float64{8.6, 47.3}, nil, encodeProps(columns, []any{"building"}))

	cases := []struct {
		name   string
		header *flat.Header
	}{
		{name: "no CRS table", header: buildHeader(flat.GeometryTypePoint, columns, 1)},
		{name: "CRS with code 0", header: buildHeaderCRS(flat.GeometryTypePoint, columns, 1, 0)},
		{name: "CRS with a negative code", header: buildHeaderCRS(flat.GeometryTypePoint, columns, 1, -1)},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			stream := streamWithHeader(t, testCase.header, []flat.Feature{feat})

			res, err := readAll(bytes.NewReader(stream))
			if err != nil {
				t.Fatalf("readAll: %v", err)
			}

			if res.EPSGCode != 0 {
				t.Fatalf("EPSGCode = %d, want 0", res.EPSGCode)
			}
		})
	}
}

// --- Typed error messages ---

// Both fault errors are what a user sees when a file is corrupt, so their text
// has to identify the file position and the faulting site rather than read as
// an internal panic dump.
func TestCorruptErrorMessages(t *testing.T) {
	t.Parallel()

	headerErr := &CorruptHeaderError{Site: "github.com/google/flatbuffers/go.(*Table).Offset", Value: "index out of range"}

	headerText := headerErr.Error()
	if !strings.Contains(headerText, "header is not a decodable FlatBuffer") {
		t.Fatalf("CorruptHeaderError.Error() = %q", headerText)
	}

	if !strings.Contains(headerText, headerErr.Site) || !strings.Contains(headerText, "index out of range") {
		t.Fatalf("CorruptHeaderError.Error() = %q, which drops the site or the value", headerText)
	}

	featureErr := &CorruptFeatureError{Index: 17, Site: "github.com/gogama/flatgeobuf/flatgeobuf.readFeature", Value: 42}

	featureText := featureErr.Error()
	if !strings.Contains(featureText, "feature 17") {
		t.Fatalf("CorruptFeatureError.Error() = %q, which does not name the feature index", featureText)
	}

	if !strings.Contains(featureText, featureErr.Site) || !strings.Contains(featureText, "42") {
		t.Fatalf("CorruptFeatureError.Error() = %q, which drops the site or the value", featureText)
	}

	// Both must be reachable with errors.As from a wrapped error, which is how
	// readAll returns them.
	var target *CorruptFeatureError
	if !errors.As(error(featureErr), &target) {
		t.Fatal("CorruptFeatureError is not matchable with errors.As")
	}
}

// buildGeometrylessFeature creates a Feature that carries properties but no
// geometry table, which FlatGeobuf permits for attribute-only records.
func buildGeometrylessFeature(props []byte) flat.Feature {
	bldr := flatbuffers.NewBuilder(256)

	var propsOff flatbuffers.UOffsetT

	if len(props) > 0 {
		propsOff = bldr.CreateByteVector(props)
	}

	flat.FeatureStart(bldr)

	if len(props) > 0 {
		flat.FeatureAddProperties(bldr, propsOff)
	}

	featOff := flat.FeatureEnd(bldr)

	bldr.FinishSizePrefixed(featOff)

	return *flat.GetSizePrefixedRootAsFeature(bldr.FinishedBytes(), 0)
}

// A feature without a geometry table is an attribute-only record. It is skipped
// rather than turned into a feature with a null geometry — but it still counts
// towards the header's declared feature count, so the integrity check must not
// then report the file as truncated.
func TestReadAllSkipsGeometrylessFeaturesWithoutFailingTheCountCheck(t *testing.T) {
	t.Parallel()

	columns := []testColumn{{name: "kind", colType: flat.ColumnTypeString}}
	props := encodeProps(columns, []any{"building"})

	features := []flat.Feature{
		buildFeature(flat.GeometryTypePoint, []float64{8.6, 47.3}, nil, props),
		buildGeometrylessFeature(props),
		buildFeature(flat.GeometryTypePoint, []float64{9.1, 48.0}, nil, props),
	}

	stream := buildTestFGBBytes(t, flat.GeometryTypePoint, columns, features)

	res, err := readAll(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}

	if len(res.Collection.Features) != 2 {
		t.Fatalf("read %d features, want the 2 that carry a geometry", len(res.Collection.Features))
	}

	for i, feat := range res.Collection.Features {
		if feat.Geometry.Type != "Point" {
			t.Fatalf("feature %d has geometry type %q", i, feat.Geometry.Type)
		}
	}
}

// A property block that cannot be read fails the whole feature rather than
// producing a geometry with silently missing attributes: kind and height_m are
// what the model is built from.
func TestConvertFeatureFailsWhenPropertiesCannotBeRead(t *testing.T) {
	t.Parallel()

	columns := []testColumn{{name: "kind", colType: flat.ColumnTypeString}}

	// A column index the schema does not declare.
	bad := []byte{0x05, 0x00, 0x00, 0x00, 0x00, 0x00}

	feat := buildFeature(flat.GeometryTypePoint, []float64{8.6, 47.3}, nil, bad)
	hdr := buildHeader(flat.GeometryTypePoint, columns, 1)

	_, ok, err := convertFeature(&feat, hdr, flat.GeometryTypePoint, 0)
	if err == nil {
		t.Fatalf("expected an error, ok = %t", ok)
	}

	if !strings.Contains(err.Error(), "read properties") {
		t.Fatalf("error %q does not say the properties failed", err)
	}
}

// Every multi-part decoder sizes a slice from a vector length taken out of the
// file, so each must reject a length the buffer cannot hold — not only the ones
// the earlier hardening tests happened to cover.
func TestMultiPartDecodersRejectOversizedVectorLengths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		geomType flat.GeometryType
		// field is the FlatBuffers vtable slot of the vector to corrupt:
		// 6 is Geometry.xy, 4 is Geometry.ends.
		field flatbuffers.VOffsetT
		xy    []float64
		ends  []uint32
	}{
		{name: "MultiPoint xy", geomType: flat.GeometryTypeMultiPoint, field: 6, xy: []float64{0, 0, 1, 1}},
		{name: "MultiLineString ends", geomType: flat.GeometryTypeMultiLineString, field: 4, xy: []float64{0, 0, 1, 1}, ends: []uint32{2}},
		{name: "MultiPolygon ends", geomType: flat.GeometryTypeMultiPolygon, field: 4, xy: []float64{0, 0, 1, 0, 1, 1, 0, 0}, ends: []uint32{4}},
		{name: "Polygon xy", geomType: flat.GeometryTypePolygon, field: 6, xy: []float64{0, 0, 1, 0, 1, 1, 0, 0}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			feat := buildFeature(testCase.geomType, testCase.xy, testCase.ends, nil)
			geom := geometryOf(t, feat)

			tab := geom.Table()

			field := flatbuffers.UOffsetT(tab.Offset(testCase.field))
			if field == 0 {
				t.Fatalf("test geometry has no vector in slot %d", testCase.field)
			}

			first := tab.Vector(field)
			putUint32(tab.Bytes[first-4:], 0xFFFFFFF0)

			_, _, err := geometryToGeoJSON(geom, testCase.geomType)
			if err == nil {
				t.Fatal("expected an error for a vector longer than the buffer")
			}

			if !strings.Contains(err.Error(), "buffer") {
				t.Fatalf("error %q does not report the oversized vector", err)
			}
		})
	}
}
