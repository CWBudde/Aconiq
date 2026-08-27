package gpkgimport

import (
	"encoding/binary"
	"strings"
	"testing"
)

// wkbHeader emits the 5-byte little-endian WKB header for a geometry type.
func wkbHeader(geomType uint32) []byte {
	h := make([]byte, wkbHeaderSize)
	h[0] = 1 // little endian
	binary.LittleEndian.PutUint32(h[1:], geomType)

	return h
}

func le32(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)

	return b
}

// A Polygon that claims 2^32-1 rings used to size a []any from the count
// before any bounds check: 17 bytes of input, 68 GB of allocation.
func TestDecodeGPKGBlob_PolygonRingCountIsBounded(t *testing.T) {
	blob := gpkgBlob(append(wkbHeader(wkbPolygon), le32(0xFFFFFFFF)...))

	if len(blob) > 32 {
		t.Fatalf("proof-of-concept blob grew to %d bytes", len(blob))
	}

	_, _, err := DecodeGPKGBlob(blob)
	if err == nil {
		t.Fatal("expected an error for a polygon declaring 2^32-1 rings")
	}

	if !strings.Contains(err.Error(), "rings") {
		t.Fatalf("error %q does not mention the ring count", err)
	}
}

func TestDecodeGPKGBlob_MultiPartCountIsBounded(t *testing.T) {
	for _, tc := range []struct {
		name     string
		geomType uint32
	}{
		{"MultiPoint", wkbMultiPoint},
		{"MultiLineString", wkbMultiLineString},
		{"MultiPolygon", wkbMultiPolygon},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blob := gpkgBlob(append(wkbHeader(tc.geomType), le32(0xFFFFFFFF)...))

			_, _, err := DecodeGPKGBlob(blob)
			if err == nil {
				t.Fatal("expected an error for a collection declaring 2^32-1 parts")
			}

			if !strings.Contains(err.Error(), "parts") {
				t.Fatalf("error %q does not mention the part count", err)
			}
		})
	}
}

func TestDecodeGPKGBlob_LineStringPointCountIsBounded(t *testing.T) {
	blob := gpkgBlob(append(wkbHeader(wkbLineString), le32(0xFFFFFFFF)...))

	_, _, err := DecodeGPKGBlob(blob)
	if err == nil {
		t.Fatal("expected an error for a linestring declaring 2^32-1 points")
	}
}

// Nested collections recurse; without a depth cap a small blob exhausts the
// goroutine stack, which is a fatal runtime error rather than a recoverable
// one.
func TestDecodeGPKGBlob_NestingDepthIsBounded(t *testing.T) {
	// Each level is a MultiPolygon holding exactly one part: 9 bytes.
	wkb := make([]byte, 0, (maxWKBNestingDepth+6)*(wkbHeaderSize+4))
	for range maxWKBNestingDepth + 5 {
		wkb = append(wkb, wkbHeader(wkbMultiPolygon)...)
		wkb = append(wkb, le32(1)...)
	}

	// Innermost part: an empty polygon.
	wkb = append(wkb, wkbHeader(wkbPolygon)...)
	wkb = append(wkb, le32(0)...)

	_, _, err := DecodeGPKGBlob(gpkgBlob(wkb))
	if err == nil {
		t.Fatal("expected an error for a deeply nested geometry")
	}

	if !strings.Contains(err.Error(), "nesting") {
		t.Fatalf("error %q does not mention nesting depth", err)
	}
}

// The bounds must not reject geometries that are simply well formed.
func TestDecodeGPKGBlob_ValidGeometriesStillDecode(t *testing.T) {
	point := append(wkbHeader(wkbPoint), make([]byte, 16)...)

	geomType, coords, err := DecodeGPKGBlob(gpkgBlob(point))
	if err != nil {
		t.Fatalf("decode point: %v", err)
	}

	if geomType != "Point" || coords == nil {
		t.Fatalf("geomType = %q, coords = %v", geomType, coords)
	}

	// A one-ring polygon with a single (degenerate) point.
	poly := wkbHeader(wkbPolygon)
	poly = append(poly, le32(1)...)
	poly = append(poly, le32(1)...)
	poly = append(poly, make([]byte, 16)...)

	geomType, coords, err = DecodeGPKGBlob(gpkgBlob(poly))
	if err != nil {
		t.Fatalf("decode polygon: %v", err)
	}

	if geomType != "Polygon" || coords == nil {
		t.Fatalf("geomType = %q, coords = %v", geomType, coords)
	}
}

func FuzzDecodeGPKGBlob(f *testing.F) {
	f.Add(gpkgBlob(append(wkbHeader(wkbPoint), make([]byte, 16)...)))
	f.Add(gpkgBlob(append(wkbHeader(wkbPolygon), le32(0xFFFFFFFF)...)))
	f.Add(gpkgBlob(append(wkbHeader(wkbMultiPolygon), le32(0xFFFFFFFF)...)))
	f.Add(gpkgBlob(append(wkbHeader(wkbLineString), le32(2)...)))
	f.Add([]byte{gpkgMagic0, gpkgMagic1, 0, 0})

	f.Fuzz(func(_ *testing.T, blob []byte) {
		// Any error is fine; panics and runaway allocations are not.
		_, _, _ = DecodeGPKGBlob(blob)
	})
}
