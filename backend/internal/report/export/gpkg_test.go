package export

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/report/results"
)

func TestExportReceiverGeoPackage(t *testing.T) {
	t.Parallel()

	table := results.ReceiverTable{
		IndicatorOrder: []string{"Lden", "Lnight"},
		Unit:           "dB",
		Records: []results.ReceiverRecord{
			{ID: "rx-001", X: 100, Y: 200, HeightM: 4, Values: map[string]float64{"Lden": 56.3, "Lnight": 47.8}},
			{ID: "rx-002", X: 110, Y: 200, HeightM: 4, Values: map[string]float64{"Lden": 58.1, "Lnight": 49.2}},
			{ID: "rx-003", X: 120, Y: 205, HeightM: 4, Values: map[string]float64{"Lden": 55.4, "Lnight": 46.6}},
		},
	}

	outDir := t.TempDir()
	gpkgPath := filepath.Join(outDir, "receivers.gpkg")

	err := ExportReceiverGeoPackage(gpkgPath, table, "EPSG:25832", 25832)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	_, err = os.Stat(gpkgPath)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	// Open and verify the GeoPackage.
	db, err := sql.Open("sqlite", gpkgPath)
	if err != nil {
		t.Fatalf("open gpkg: %v", err)
	}
	defer db.Close()

	// Verify application_id.
	var appID int

	err = db.QueryRow("PRAGMA application_id").Scan(&appID)
	if err != nil {
		t.Fatalf("query application_id: %v", err)
	}

	if appID != 0x47504B47 {
		t.Fatalf("application_id = 0x%X, want 0x47504B47", appID)
	}

	// Verify gpkg_contents has the receivers table.
	var tableName string

	err = db.QueryRow("SELECT table_name FROM gpkg_contents WHERE table_name = 'receivers'").Scan(&tableName)
	if err != nil {
		t.Fatalf("query gpkg_contents: %v", err)
	}

	// Verify receiver count.
	var count int

	err = db.QueryRow("SELECT COUNT(*) FROM receivers").Scan(&count)
	if err != nil {
		t.Fatalf("query receiver count: %v", err)
	}

	if count != 3 {
		t.Fatalf("receiver count = %d, want 3", count)
	}

	// Verify indicator columns exist and have correct values.
	var lden, lnight float64

	err = db.QueryRow("SELECT lden, lnight FROM receivers WHERE receiver_id = 'rx-001'").Scan(&lden, &lnight)
	if err != nil {
		t.Fatalf("query rx-001: %v", err)
	}

	if lden != 56.3 {
		t.Fatalf("rx-001 lden = %v, want 56.3", lden)
	}

	if lnight != 47.8 {
		t.Fatalf("rx-001 lnight = %v, want 47.8", lnight)
	}

	// Verify geometry column is registered.
	var geomType string

	err = db.QueryRow("SELECT geometry_type_name FROM gpkg_geometry_columns WHERE table_name = 'receivers'").Scan(&geomType)
	if err != nil {
		t.Fatalf("query geometry column: %v", err)
	}

	if geomType != "POINT" {
		t.Fatalf("geometry_type = %q, want POINT", geomType)
	}

	// Verify geom blobs are not null.
	var geomBlob []byte

	err = db.QueryRow("SELECT geom FROM receivers WHERE receiver_id = 'rx-001'").Scan(&geomBlob)
	if err != nil {
		t.Fatalf("query geom: %v", err)
	}

	if len(geomBlob) == 0 {
		t.Fatal("geom blob is empty")
	}

	// Verify GP header.
	if geomBlob[0] != 'G' || geomBlob[1] != 'P' {
		t.Fatalf("geom blob header = %c%c, want GP", geomBlob[0], geomBlob[1])
	}
}

func TestExportContourGeoPackage(t *testing.T) {
	t.Parallel()

	contours := []ContourLine{
		{
			Level:    50,
			BandName: "Lden",
			Points:   [][2]float64{{100, 200}, {110, 210}, {120, 200}},
		},
		{
			Level:    55,
			BandName: "Lden",
			Points:   [][2]float64{{105, 205}, {115, 215}},
		},
	}

	outDir := t.TempDir()
	gpkgPath := filepath.Join(outDir, "contours.gpkg")

	err := ExportContourGeoPackage(gpkgPath, contours, "EPSG:25832", 25832)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	db, err := sql.Open("sqlite", gpkgPath)
	if err != nil {
		t.Fatalf("open gpkg: %v", err)
	}
	defer db.Close()

	var count int

	err = db.QueryRow("SELECT COUNT(*) FROM contours").Scan(&count)
	if err != nil {
		t.Fatalf("query contour count: %v", err)
	}

	if count != 2 {
		t.Fatalf("contour count = %d, want 2", count)
	}

	var levelDB float64

	err = db.QueryRow("SELECT level_db FROM contours ORDER BY level_db LIMIT 1").Scan(&levelDB)
	if err != nil {
		t.Fatalf("query first contour: %v", err)
	}

	if levelDB != 50 {
		t.Fatalf("first contour level = %v, want 50", levelDB)
	}
}

func TestExportContourGeoPackageEmpty(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	gpkgPath := filepath.Join(outDir, "empty.gpkg")

	err := ExportContourGeoPackage(gpkgPath, nil, "EPSG:25832", 25832)
	if err != nil {
		t.Fatalf("export empty: %v", err)
	}

	db, err := sql.Open("sqlite", gpkgPath)
	if err != nil {
		t.Fatalf("open gpkg: %v", err)
	}
	defer db.Close()

	var count int

	err = db.QueryRow("SELECT COUNT(*) FROM contours").Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}

func TestExportModelFeaturesGeoPackage(t *testing.T) {
	t.Parallel()

	features := []ModelFeature{
		{ID: "src-1", Kind: "source", SourceType: "point", GeometryType: "Point", Coordinates: []any{100.0, 200.0}},
		{ID: "bld-1", Kind: "building", HeightM: 12.0, GeometryType: "Polygon", Coordinates: []any{[]any{[]any{100.0, 200.0}, []any{110.0, 200.0}, []any{110.0, 210.0}, []any{100.0, 210.0}, []any{100.0, 200.0}}}},
		{ID: "bar-1", Kind: "barrier", HeightM: 2.5, GeometryType: "LineString", Coordinates: []any{[]any{100.0, 200.0}, []any{120.0, 200.0}}},
	}

	outDir := t.TempDir()
	gpkgPath := filepath.Join(outDir, "model.gpkg")

	err := ExportModelFeaturesGeoPackage(gpkgPath, features, "EPSG:25832", 25832)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	_, err = os.Stat(gpkgPath)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	db, err := sql.Open("sqlite", gpkgPath)
	if err != nil {
		t.Fatalf("open gpkg: %v", err)
	}
	defer db.Close()

	// Verify application_id.
	var appID int

	err = db.QueryRow("PRAGMA application_id").Scan(&appID)
	if err != nil {
		t.Fatalf("query application_id: %v", err)
	}

	if appID != 0x47504B47 {
		t.Fatalf("application_id = 0x%X, want 0x47504B47", appID)
	}

	// Verify gpkg_contents has the model_features table.
	var tableName string

	err = db.QueryRow("SELECT table_name FROM gpkg_contents WHERE table_name = 'model_features'").Scan(&tableName)
	if err != nil {
		t.Fatalf("query gpkg_contents: %v", err)
	}

	// Verify feature count.
	var count int

	err = db.QueryRow("SELECT COUNT(*) FROM model_features").Scan(&count)
	if err != nil {
		t.Fatalf("query feature count: %v", err)
	}

	if count != 3 {
		t.Fatalf("feature count = %d, want 3", count)
	}

	// Verify geometry column is registered as GEOMETRY (mixed types).
	var geomType string

	err = db.QueryRow("SELECT geometry_type_name FROM gpkg_geometry_columns WHERE table_name = 'model_features'").Scan(&geomType)
	if err != nil {
		t.Fatalf("query geometry column: %v", err)
	}

	if geomType != "GEOMETRY" {
		t.Fatalf("geometry_type = %q, want GEOMETRY", geomType)
	}

	// Verify columns and values for a specific feature.
	var featureID, kind, sourceType string
	var heightM float64

	err = db.QueryRow("SELECT feature_id, kind, source_type, height_m FROM model_features WHERE feature_id = 'bar-1'").Scan(&featureID, &kind, &sourceType, &heightM)
	if err != nil {
		t.Fatalf("query bar-1: %v", err)
	}

	if kind != "barrier" {
		t.Fatalf("bar-1 kind = %q, want barrier", kind)
	}

	if heightM != 2.5 {
		t.Fatalf("bar-1 height_m = %v, want 2.5", heightM)
	}

	// Verify geom blobs are not null.
	var geomBlob []byte

	err = db.QueryRow("SELECT geom FROM model_features WHERE feature_id = 'src-1'").Scan(&geomBlob)
	if err != nil {
		t.Fatalf("query geom: %v", err)
	}

	if len(geomBlob) == 0 {
		t.Fatal("geom blob is empty")
	}

	// Verify GP header.
	if geomBlob[0] != 'G' || geomBlob[1] != 'P' {
		t.Fatalf("geom blob header = %c%c, want GP", geomBlob[0], geomBlob[1])
	}
}

func TestExportModelFeaturesGeoPackageEmpty(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	gpkgPath := filepath.Join(outDir, "empty-model.gpkg")

	err := ExportModelFeaturesGeoPackage(gpkgPath, nil, "EPSG:25832", 25832)
	if err != nil {
		t.Fatalf("export empty: %v", err)
	}

	db, err := sql.Open("sqlite", gpkgPath)
	if err != nil {
		t.Fatalf("open gpkg: %v", err)
	}
	defer db.Close()

	var count int

	err = db.QueryRow("SELECT COUNT(*) FROM model_features").Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}

func TestSanitizeColumnName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"Lden", "lden"},
		{"Lnight", "lnight"},
		{"L_day", "l_day"},
		{"LrDay", "lrday"},
		{"123abc", "v_123abc"},
		{"a-b/c", "a_b_c"},
		{"", "value"},
	}

	for _, tt := range tests {
		got := sanitizeColumnName(tt.input)
		if got != tt.want {
			t.Fatalf("sanitizeColumnName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
