package fgbimport

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/gogama/flatgeobuf/flatgeobuf/flat"
	flatbuffers "github.com/google/flatbuffers/go"
)

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
