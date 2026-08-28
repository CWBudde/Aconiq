package fgbimport

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/gogama/flatgeobuf/flatgeobuf"
	"github.com/gogama/flatgeobuf/flatgeobuf/flat"
	flatbuffers "github.com/google/flatbuffers/go"
)

// fgbMagicLen is the length of the FlatGeobuf magic number that precedes the
// size-prefixed header. The reader library keeps its copy unexported.
const fgbMagicLen = 8

// geometryOf extracts the geometry from a test feature.
func geometryOf(t *testing.T, feat flat.Feature) *flat.Geometry {
	t.Helper()

	geom := new(flat.Geometry)
	if feat.Geometry(geom) == nil {
		t.Fatal("test feature has no geometry")
	}

	return geom
}

// The ends vector says where each ring stops. Its entries are 32-bit values
// straight out of the file, and the ring decoder used to size a []any from
// end-start without checking either against the coordinates actually present:
// one ends entry of 2^32-1 asked for a 68 GB slice.
func TestGeometryToGeoJSON_RejectsRingEndBeyondCoordinates(t *testing.T) {
	feat := buildFeature(flat.GeometryTypePolygon, []float64{0, 0, 1, 0, 1, 1, 0, 0}, []uint32{0xFFFFFFFF}, nil)

	_, _, err := geometryToGeoJSON(geometryOf(t, feat), flat.GeometryTypePolygon)
	if err == nil {
		t.Fatal("expected an error for a ring end beyond the coordinate vector")
	}

	if !strings.Contains(err.Error(), "coordinate range") {
		t.Fatalf("error %q does not report the bad coordinate range", err)
	}
}

// Ends must be non-decreasing; a decreasing pair yields a negative length,
// which make() turns into a panic rather than an error.
func TestGeometryToGeoJSON_RejectsDecreasingRingEnds(t *testing.T) {
	feat := buildFeature(flat.GeometryTypePolygon, []float64{0, 0, 1, 0, 1, 1, 0, 0}, []uint32{4, 2}, nil)

	_, _, err := geometryToGeoJSON(geometryOf(t, feat), flat.GeometryTypePolygon)
	if err == nil {
		t.Fatal("expected an error for decreasing ring ends")
	}
}

func TestGeometryToGeoJSON_RejectsLineEndBeyondCoordinates(t *testing.T) {
	feat := buildFeature(flat.GeometryTypeMultiLineString, []float64{0, 0, 1, 1}, []uint32{9999}, nil)

	_, _, err := geometryToGeoJSON(geometryOf(t, feat), flat.GeometryTypeMultiLineString)
	if err == nil {
		t.Fatal("expected an error for a linestring end beyond the coordinate vector")
	}
}

// The FlatGeobuf reader runs no FlatBuffers verifier, so a vector's own length
// prefix is untrusted too: it must be checked against the buffer that holds it
// before it sizes a slice or indexes into the data.
func TestGeometryToGeoJSON_RejectsVectorLengthBeyondBuffer(t *testing.T) {
	feat := buildFeature(flat.GeometryTypeLineString, []float64{0, 0, 1, 1}, nil, nil)
	geom := geometryOf(t, feat)

	tab := geom.Table()

	xyField := flatbuffers.UOffsetT(tab.Offset(6)) // Geometry.xy
	if xyField == 0 {
		t.Fatal("test geometry has no xy vector")
	}

	// The vector's length prefix sits in the four bytes before its first element.
	first := tab.Vector(xyField)
	binary.LittleEndian.PutUint32(tab.Bytes[first-4:], 0xFFFFFFF0)

	_, _, err := geometryToGeoJSON(geom, flat.GeometryTypeLineString)
	if err == nil {
		t.Fatal("expected an error for an xy vector longer than the buffer")
	}

	if !strings.Contains(err.Error(), "buffer") {
		t.Fatalf("error %q does not report the oversized vector", err)
	}
}

// Well-formed geometries must still decode.
func TestGeometryToGeoJSON_ValidPolygonStillDecodes(t *testing.T) {
	feat := buildFeature(flat.GeometryTypePolygon, []float64{0, 0, 1, 0, 1, 1, 0, 0}, []uint32{4}, nil)

	geomType, coords, err := geometryToGeoJSON(geometryOf(t, feat), flat.GeometryTypePolygon)
	if err != nil {
		t.Fatalf("decode valid polygon: %v", err)
	}

	if geomType != "Polygon" {
		t.Fatalf("geomType = %q, want Polygon", geomType)
	}

	rings, ok := coords.([]any)
	if !ok || len(rings) != 1 {
		t.Fatalf("coords = %#v, want one ring", coords)
	}
}

// buildTestFGBBytes assembles a complete in-memory FlatGeobuf stream.
func buildTestFGBBytes(t testing.TB, geomType flat.GeometryType, columns []testColumn, features []flat.Feature) []byte {
	t.Helper()

	var buf bytes.Buffer

	w := flatgeobuf.NewFileWriter(&buf)

	_, err := w.Header(buildHeader(geomType, columns, len(features)))
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

// setHeaderFeatureCount rewrites the features_count field of an assembled
// FlatGeobuf stream, returning a copy.
func setHeaderFeatureCount(t testing.TB, stream []byte, count uint64) []byte {
	t.Helper()

	out := bytes.Clone(stream)

	// The size-prefixed header follows the magic number, and shares its
	// backing array with out, so the mutation lands in the returned stream.
	hdr := flat.GetSizePrefixedRootAsHeader(out[fgbMagicLen:], 0)
	if !hdr.MutateFeaturesCount(count) {
		t.Fatal("test header has no features_count field to mutate")
	}

	return out
}

// setFirstFeatureLength rewrites the length prefix of the first feature in an
// assembled FlatGeobuf stream, returning a copy.
func setFirstFeatureLength(t testing.TB, stream []byte, length uint32) []byte {
	t.Helper()

	out := bytes.Clone(stream)

	// The data section follows the magic number and the size-prefixed header.
	dataOffset := fgbMagicLen + 4 + int(binary.LittleEndian.Uint32(out[fgbMagicLen:]))
	if dataOffset+4 > len(out) {
		t.Fatal("test stream has no data section")
	}

	binary.LittleEndian.PutUint32(out[dataOffset:], length)

	return out
}

// The FlatGeobuf header declares how many features the data section holds, in
// an unvalidated uint64. FileReader.DataRem sizes one []flat.Feature from that
// number, so a 65-byte file used to ask the allocator for a terabyte and take
// the process down with it. Reading in fixed-size chunks means the header
// cannot size an allocation at all.
func TestReadAll_HeaderFeatureCountDoesNotSizeAnAllocation(t *testing.T) {
	columns := []testColumn{{name: "kind", colType: flat.ColumnTypeString}}
	feat := buildFeature(flat.GeometryTypePoint, []float64{8.6, 47.3}, nil, encodeProps(columns, []any{"building"}))

	stream := buildTestFGBBytes(t, flat.GeometryTypePoint, columns, []flat.Feature{feat})
	if len(stream) > 4096 {
		t.Fatalf("proof-of-concept stream grew to %d bytes", len(stream))
	}

	lying := setHeaderFeatureCount(t, stream, 1<<40)

	// The read must fail rather than allocate 2^40 feature structures.
	_, err := readAll(bytes.NewReader(lying))
	if err == nil {
		t.Fatal("expected an error for a header declaring more features than the file holds")
	}
}

// A header whose declared count is honest must still read.
func TestReadAll_HonestFeatureCountStillReads(t *testing.T) {
	columns := []testColumn{{name: "kind", colType: flat.ColumnTypeString}}
	feat := buildFeature(flat.GeometryTypePoint, []float64{8.6, 47.3}, nil, encodeProps(columns, []any{"building"}))

	stream := buildTestFGBBytes(t, flat.GeometryTypePoint, columns, []flat.Feature{feat})

	res, err := readAll(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}

	if len(res.Collection.Features) != 1 {
		t.Fatalf("read %d features, want 1", len(res.Collection.Features))
	}
}

// Many features must all come through, in order.
func TestReadFeatures_ReadsEveryFeatureInOrder(t *testing.T) {
	columns := []testColumn{{name: "kind", colType: flat.ColumnTypeString}}
	props := encodeProps(columns, []any{"building"})

	const count = 1031

	features := make([]flat.Feature, 0, count)
	for i := range count {
		features = append(features, buildFeature(flat.GeometryTypePoint, []float64{float64(i), 47.3}, nil, props))
	}

	stream := buildTestFGBBytes(t, flat.GeometryTypePoint, columns, features)

	res, err := readAll(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}

	if len(res.Collection.Features) != count {
		t.Fatalf("read %d features, want %d", len(res.Collection.Features), count)
	}

	for i, feat := range res.Collection.Features {
		coords, ok := feat.Geometry.Coordinates.([]any)
		if !ok || len(coords) != 2 {
			t.Fatalf("feature %d: coordinates = %#v", i, feat.Geometry.Coordinates)
		}

		if coords[0] != float64(i) {
			t.Fatalf("feature %d: x = %v, want %d", i, coords[0], i)
		}
	}
}

// A fault raised inside the FlatBuffers reader becomes a typed error naming the
// feature, not a crash.
func TestDecodeFeature_TurnsLibraryFaultIntoTypedError(t *testing.T) {
	feat := buildFeature(flat.GeometryTypePoint, []float64{1, 2}, nil, nil)

	// Point the feature table's vtable offset far outside the buffer, which is
	// exactly the corruption the reader library does not check for.
	tab := feat.Table()
	binary.LittleEndian.PutUint32(tab.Bytes[tab.Pos:], 0x7FFFFFF0)

	hdr := buildHeader(flat.GeometryTypePoint, nil, 1)

	_, err := decodeFeature(&feat, hdr, flat.GeometryTypePoint, 3)
	if err == nil {
		t.Fatal("expected an error for a feature with a vtable outside the buffer")
	}

	var corrupt *CorruptFeatureError
	if !errors.As(err, &corrupt) {
		t.Fatalf("error %v is not a *CorruptFeatureError", err)
	}

	if corrupt.Index != 3 {
		t.Fatalf("CorruptFeatureError.Index = %d, want 3", corrupt.Index)
	}

	if corrupt.Value == nil {
		t.Fatal("CorruptFeatureError discarded the panic value")
	}

	if strings.HasPrefix(corrupt.Site, aconiqPackagePrefix) {
		t.Fatalf("CorruptFeatureError.Site = %q, which is this repository's own code", corrupt.Site)
	}
}

// The guard must not be able to absorb a fault raised by this repository's own
// code: that would turn an Aconiq bug into a silently skipped file.
func TestPanicSite_IdentifiesAconiqCode(t *testing.T) {
	site := func() (site string) {
		defer func() {
			_ = recover()

			site = panicSite()
		}()

		panicFromAconiqCode()

		return ""
	}()

	if !strings.HasPrefix(site, aconiqPackagePrefix) {
		t.Fatalf("panicSite() = %q, want a %s… frame", site, aconiqPackagePrefix)
	}
}

//go:noinline
func panicFromAconiqCode() {
	var empty []int

	_ = empty[1]
}

// The reader library does not verify the header table either, so a corrupt
// header vtable faults in hdr.GeometryType() before any feature is reached.
func TestReadHeaderFields_TurnsLibraryFaultIntoTypedError(t *testing.T) {
	hdr := buildHeader(flat.GeometryTypePoint, nil, 0)

	// Point the header table's vtable offset far outside the buffer.
	tab := hdr.Table()
	binary.LittleEndian.PutUint32(tab.Bytes[tab.Pos:], 0x7FFFFFF0)

	_, err := readHeaderFields(hdr)
	if err == nil {
		t.Fatal("expected an error for a header with a vtable outside the buffer")
	}

	var corrupt *CorruptHeaderError
	if !errors.As(err, &corrupt) {
		t.Fatalf("error %v is not a *CorruptHeaderError", err)
	}

	if corrupt.Value == nil {
		t.Fatal("CorruptHeaderError discarded the panic value")
	}

	if strings.HasPrefix(corrupt.Site, aconiqPackagePrefix) {
		t.Fatalf("CorruptHeaderError.Site = %q, which is this repository's own code", corrupt.Site)
	}
}

// The reader library owns everything up to the first feature, including
// stepping over the packed R-tree index. Every other fixture in this package
// declares no index, so this is the only test that proves the hand-off point
// is right for the files GDAL actually writes.
func TestReadAll_SkipsSpatialIndex(t *testing.T) {
	columns := []testColumn{{name: "kind", colType: flat.ColumnTypeString}}
	props := encodeProps(columns, []any{"building"})

	features := []flat.Feature{
		buildFeature(flat.GeometryTypePoint, []float64{8.6, 47.3}, nil, props),
		buildFeature(flat.GeometryTypePoint, []float64{9.1, 48.0}, nil, props),
		buildFeature(flat.GeometryTypePoint, []float64{10.2, 49.4}, nil, props),
	}

	var buf bytes.Buffer

	w := flatgeobuf.NewFileWriter(&buf)

	_, err := w.Header(buildHeaderNodeSize(flat.GeometryTypePoint, columns, len(features), 16))
	if err != nil {
		t.Fatalf("write header: %v", err)
	}

	_, err = w.IndexData(features)
	if err != nil {
		t.Fatalf("write index and data: %v", err)
	}

	res, err := readAll(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}

	if len(res.Collection.Features) != len(features) {
		t.Fatalf("read %d features, want %d", len(res.Collection.Features), len(features))
	}
}

// Each feature buffer is prefixed with its length as an unvalidated uint32, and
// the reader library sizes the buffer from it directly:
//
//	tbl := make([]byte, flatbuffers.SizeUint32+featureLen)
//
// A 116-byte file that declares a 4 GiB feature therefore cost 4.2 GB of RSS
// before it failed, which is what made FuzzReadFGB unrunnable in parallel. The
// length has to be checked against the bytes still in the file first.
func TestReadFeatures_RejectsFeatureLengthBeyondTheDataSection(t *testing.T) {
	columns := []testColumn{{name: "kind", colType: flat.ColumnTypeString}}
	feat := buildFeature(flat.GeometryTypePoint, []float64{8.6, 47.3}, nil, encodeProps(columns, []any{"building"}))

	stream := buildTestFGBBytes(t, flat.GeometryTypePoint, columns, []flat.Feature{feat})
	if len(stream) > 4096 {
		t.Fatalf("proof-of-concept stream grew to %d bytes", len(stream))
	}

	// The data section follows the magic number and the size-prefixed header.
	dataOffset := fgbMagicLen + 4 + int(binary.LittleEndian.Uint32(stream[fgbMagicLen:]))

	lying := bytes.Clone(stream)
	binary.LittleEndian.PutUint32(lying[dataOffset:], 0xFFFFFFF0)

	_, err := readAll(bytes.NewReader(lying))
	if err == nil {
		t.Fatal("expected an error for a feature longer than the data section")
	}

	if !strings.Contains(err.Error(), "declared length") {
		t.Fatalf("error %q does not report the bad declared length", err)
	}
}

// A length prefix below the FlatBuffers minimum is malformed, and rejecting it
// keeps the read from looping on a zero-length feature.
func TestReadFeatures_RejectsUndersizedFeatureLength(t *testing.T) {
	columns := []testColumn{{name: "kind", colType: flat.ColumnTypeString}}
	feat := buildFeature(flat.GeometryTypePoint, []float64{8.6, 47.3}, nil, encodeProps(columns, []any{"building"}))

	stream := buildTestFGBBytes(t, flat.GeometryTypePoint, columns, []flat.Feature{feat})
	dataOffset := fgbMagicLen + 4 + int(binary.LittleEndian.Uint32(stream[fgbMagicLen:]))

	lying := bytes.Clone(stream)
	binary.LittleEndian.PutUint32(lying[dataOffset:], minFeatureBytes-1)

	_, err := readAll(bytes.NewReader(lying))
	if err == nil {
		t.Fatal("expected an error for a feature shorter than a FlatBuffers root offset")
	}
}

func FuzzReadFGB(f *testing.F) {
	columns := []testColumn{
		{name: "kind", colType: flat.ColumnTypeString},
		{name: "height_m", colType: flat.ColumnTypeDouble},
	}
	props := encodeProps(columns, []any{"building", 12.5})

	point := buildFeature(flat.GeometryTypePoint, []float64{8.6, 47.3}, nil, props)
	line := buildFeature(flat.GeometryTypeLineString, []float64{0, 0, 1, 1, 2, 0}, nil, props)
	polygon := buildFeature(flat.GeometryTypePolygon, []float64{0, 0, 1, 0, 1, 1, 0, 1, 0, 0}, []uint32{5}, props)

	pointStream := buildTestFGBBytes(f, flat.GeometryTypePoint, columns, []flat.Feature{point})

	f.Add(pointStream)
	f.Add(buildTestFGBBytes(f, flat.GeometryTypeLineString, columns, []flat.Feature{line}))
	f.Add(buildTestFGBBytes(f, flat.GeometryTypePolygon, columns, []flat.Feature{polygon}))
	f.Add(buildTestFGBBytes(f, flat.GeometryTypePoint, nil, nil))
	// A header that claims far more features than the file can hold, which
	// used to size a terabyte-scale []flat.Feature.
	f.Add(setHeaderFeatureCount(f, pointStream, 1<<40))
	// A feature that claims to be 4 GiB long, which used to size a 4 GiB
	// buffer out of a 116-byte file.
	f.Add(setFirstFeatureLength(f, pointStream, 0xFFFFFFF0))
	f.Add(pointStream[:fgbMagicLen])
	f.Add([]byte{})

	f.Fuzz(func(_ *testing.T, data []byte) {
		// Any error is acceptable; panics and runaway allocations are not.
		_, _ = readAll(bytes.NewReader(data))
	})
}
