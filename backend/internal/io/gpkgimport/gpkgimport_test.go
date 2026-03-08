package gpkgimport

import (
	"database/sql"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// --- WKB blob helpers ---

func makePoint(x, y float64) []byte {
	buf := make([]byte, 21) // 1+4+8+8
	buf[0] = 1              // little-endian
	binary.LittleEndian.PutUint32(buf[1:], wkbPoint)
	binary.LittleEndian.PutUint64(buf[5:], math.Float64bits(x))
	binary.LittleEndian.PutUint64(buf[13:], math.Float64bits(y))

	return buf
}

func makeLineString(pts [][2]float64) []byte {
	buf := make([]byte, 5+4+len(pts)*16)
	buf[0] = 1 // little-endian
	binary.LittleEndian.PutUint32(buf[1:], wkbLineString)
	binary.LittleEndian.PutUint32(buf[5:], uint32(len(pts)))

	for i, pt := range pts {
		off := 9 + i*16
		binary.LittleEndian.PutUint64(buf[off:], math.Float64bits(pt[0]))
		binary.LittleEndian.PutUint64(buf[off+8:], math.Float64bits(pt[1]))
	}

	return buf
}

func makePolygon(rings [][][2]float64) []byte {
	size := 5 + 4
	for _, r := range rings {
		size += 4 + len(r)*16
	}

	buf := make([]byte, size)
	buf[0] = 1
	binary.LittleEndian.PutUint32(buf[1:], wkbPolygon)
	binary.LittleEndian.PutUint32(buf[5:], uint32(len(rings)))

	off := 9

	for _, r := range rings {
		binary.LittleEndian.PutUint32(buf[off:], uint32(len(r)))
		off += 4

		for _, pt := range r {
			binary.LittleEndian.PutUint64(buf[off:], math.Float64bits(pt[0]))
			binary.LittleEndian.PutUint64(buf[off+8:], math.Float64bits(pt[1]))
			off += 16
		}
	}

	return buf
}

func gpkgBlob(wkb []byte) []byte {
	// Header: GP + version(1) + flags(1) + srs_id(4) = 8 bytes total, no envelope.
	hdr := []byte{0x47, 0x50, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	return append(hdr, wkb...)
}

// --- DecodeGPKGBlob tests ---

func TestDecodeGPKGBlob_Point(t *testing.T) {
	blob := gpkgBlob(makePoint(7.5, 48.0))

	gt, coords, err := DecodeGPKGBlob(blob)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gt != "Point" {
		t.Errorf("expected geomType 'Point', got %q", gt)
	}

	pts, ok := coords.([]any)
	if !ok || len(pts) != 2 {
		t.Fatalf("expected []any{x, y}, got %T %v", coords, coords)
	}

	if pts[0].(float64) != 7.5 || pts[1].(float64) != 48.0 {
		t.Errorf("unexpected coords: %v", pts)
	}
}

func TestDecodeGPKGBlob_LineString(t *testing.T) {
	pts := [][2]float64{{0, 0}, {1, 1}, {2, 0}}
	blob := gpkgBlob(makeLineString(pts))

	gt, coords, err := DecodeGPKGBlob(blob)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gt != "LineString" {
		t.Errorf("expected geomType 'LineString', got %q", gt)
	}

	arr, ok := coords.([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("expected 3 points, got %T %v", coords, coords)
	}
}

func TestDecodeGPKGBlob_Polygon(t *testing.T) {
	ring := [][2]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0}}
	blob := gpkgBlob(makePolygon([][][2]float64{ring}))

	gt, coords, err := DecodeGPKGBlob(blob)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gt != "Polygon" {
		t.Errorf("expected geomType 'Polygon', got %q", gt)
	}

	rings, ok := coords.([]any)
	if !ok || len(rings) != 1 {
		t.Fatalf("expected 1 ring, got %T %v", coords, coords)
	}
}

func TestDecodeGPKGBlob_InvalidMagic(t *testing.T) {
	blob := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	_, _, err := DecodeGPKGBlob(blob)
	if err == nil {
		t.Fatal("expected error for invalid magic, got nil")
	}
}

func TestDecodeGPKGBlob_EmptyGeometry(t *testing.T) {
	// flags byte: empty geom bit (bit 3 = 0x08) set, no envelope (bits 0-2 = 0)
	hdr := []byte{0x47, 0x50, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00}

	gt, coords, err := DecodeGPKGBlob(hdr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gt != "" || coords != nil {
		t.Errorf("expected empty geometry, got gt=%q coords=%v", gt, coords)
	}
}

// --- GeoPackage integration tests ---

func createTestGPKG(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.gpkg")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("create test gpkg: %v", err)
	}

	defer db.Close()

	_, err = db.Exec(`CREATE TABLE gpkg_contents (
		table_name TEXT NOT NULL,
		data_type TEXT NOT NULL,
		identifier TEXT,
		description TEXT NOT NULL DEFAULT '',
		last_change TEXT,
		min_x REAL, min_y REAL, max_x REAL, max_y REAL,
		srs_id INTEGER
	)`)
	if err != nil {
		t.Fatalf("create gpkg_contents: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE gpkg_geometry_columns (
		table_name TEXT NOT NULL,
		column_name TEXT NOT NULL,
		geometry_type_name TEXT NOT NULL,
		srs_id INTEGER NOT NULL,
		z TINYINT NOT NULL,
		m TINYINT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create gpkg_geometry_columns: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE noise_features (
		fid INTEGER PRIMARY KEY,
		geom BLOB,
		kind TEXT,
		height_m REAL,
		name TEXT
	)`)
	if err != nil {
		t.Fatalf("create noise_features: %v", err)
	}

	_, err = db.Exec(`INSERT INTO gpkg_contents (table_name, data_type, description)
		VALUES ('noise_features', 'features', 'Test noise features')`)
	if err != nil {
		t.Fatalf("insert gpkg_contents: %v", err)
	}

	_, err = db.Exec(`INSERT INTO gpkg_geometry_columns (table_name, column_name, geometry_type_name, srs_id, z, m)
		VALUES ('noise_features', 'geom', 'POINT', 4326, 0, 0)`)
	if err != nil {
		t.Fatalf("insert gpkg_geometry_columns: %v", err)
	}

	pointBlob := gpkgBlob(makePoint(8.6, 47.3))
	lineBlob := gpkgBlob(makeLineString([][2]float64{{0, 0}, {1, 1}}))

	_, err = db.Exec(`INSERT INTO noise_features (fid, geom, kind, height_m, name) VALUES (1, ?, 'building', 12.5, 'Bldg A')`, pointBlob)
	if err != nil {
		t.Fatalf("insert row 1: %v", err)
	}

	_, err = db.Exec(`INSERT INTO noise_features (fid, geom, kind, height_m, name) VALUES (2, ?, 'source', NULL, 'Road B')`, lineBlob)
	if err != nil {
		t.Fatalf("insert row 2: %v", err)
	}

	_, err = db.Exec(`INSERT INTO noise_features (fid, geom, kind, height_m, name) VALUES (3, NULL, 'barrier', 3.0, 'Null geom')`)
	if err != nil {
		t.Fatalf("insert row 3 (null geom): %v", err)
	}

	return path
}

func TestListLayers(t *testing.T) {
	path := createTestGPKG(t)

	layers, err := ListLayers(path)
	if err != nil {
		t.Fatalf("ListLayers: %v", err)
	}

	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}

	if layers[0].Name != "noise_features" {
		t.Errorf("expected layer 'noise_features', got %q", layers[0].Name)
	}
}

func TestReadLayer_PointAndLine(t *testing.T) {
	path := createTestGPKG(t)

	fc, err := ReadLayer(path, "noise_features")
	if err != nil {
		t.Fatalf("ReadLayer: %v", err)
	}

	if fc.Type != "FeatureCollection" {
		t.Errorf("expected FeatureCollection, got %q", fc.Type)
	}

	// Row 3 (null geom) should be skipped → 2 features.
	if len(fc.Features) != 2 {
		t.Fatalf("expected 2 features (null geom skipped), got %d", len(fc.Features))
	}

	f0 := fc.Features[0]
	if f0.Geometry.Type != "Point" {
		t.Errorf("feature 0: expected Point geometry, got %q", f0.Geometry.Type)
	}

	f1 := fc.Features[1]
	if f1.Geometry.Type != "LineString" {
		t.Errorf("feature 1: expected LineString geometry, got %q", f1.Geometry.Type)
	}
}

func TestReadLayer_NotFound(t *testing.T) {
	path := createTestGPKG(t)

	_, err := ReadLayer(path, "nonexistent_layer")
	if err == nil {
		t.Fatal("expected error for nonexistent layer, got nil")
	}
}

func TestListLayers_FileNotFound(t *testing.T) {
	_, err := ListLayers(filepath.Join(os.TempDir(), "does_not_exist.gpkg"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
