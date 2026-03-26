package export

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aconiq/backend/internal/report/results"
	_ "modernc.org/sqlite"
)

// ExportReceiverGeoPackage writes a receiver table as an OGC GeoPackage
// with attributed point features.
func ExportReceiverGeoPackage(path string, table results.ReceiverTable, crs string, srsID int) error {
	err := table.Validate()
	if err != nil {
		return fmt.Errorf("validate receiver table: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return fmt.Errorf("create gpkg directory: %w", err)
	}

	// Remove existing file to start fresh.
	_ = os.Remove(path)

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open gpkg database: %w", err)
	}
	defer db.Close()

	err = initGeoPackage(db, crs, srsID)
	if err != nil {
		return fmt.Errorf("init geopackage: %w", err)
	}

	err = createReceiverTable(db, table, srsID)
	if err != nil {
		return fmt.Errorf("create receiver table: %w", err)
	}

	err = insertReceivers(db, table)
	if err != nil {
		return fmt.Errorf("insert receivers: %w", err)
	}

	return nil
}

// ExportContourGeoPackage writes contour lines as an OGC GeoPackage.
func ExportContourGeoPackage(path string, contours []ContourLine, crs string, srsID int) error {
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return fmt.Errorf("create gpkg directory: %w", err)
	}

	_ = os.Remove(path)

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open gpkg database: %w", err)
	}
	defer db.Close()

	err = initGeoPackage(db, crs, srsID)
	if err != nil {
		return fmt.Errorf("init geopackage: %w", err)
	}

	err = createContourTable(db, srsID)
	if err != nil {
		return fmt.Errorf("create contour table: %w", err)
	}

	err = insertContours(db, contours)
	if err != nil {
		return fmt.Errorf("insert contours: %w", err)
	}

	return nil
}

func initGeoPackage(db *sql.DB, crs string, srsID int) error {
	ctx := context.Background()

	// Set GeoPackage application_id.
	_, err := db.ExecContext(ctx, "PRAGMA application_id = 0x47504B47") // 'GPKG'
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, "PRAGMA user_version = 10301") // GeoPackage 1.3.1
	if err != nil {
		return err
	}

	// Create required GeoPackage metadata tables.
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS gpkg_spatial_ref_sys (
			srs_name TEXT NOT NULL,
			srs_id INTEGER NOT NULL PRIMARY KEY,
			organization TEXT NOT NULL,
			organization_coordsys_id INTEGER NOT NULL,
			definition TEXT NOT NULL,
			description TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS gpkg_contents (
			table_name TEXT NOT NULL PRIMARY KEY,
			data_type TEXT NOT NULL,
			identifier TEXT,
			description TEXT DEFAULT '',
			last_change DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			min_x DOUBLE,
			min_y DOUBLE,
			max_x DOUBLE,
			max_y DOUBLE,
			srs_id INTEGER,
			CONSTRAINT fk_gc_r_srs_id FOREIGN KEY (srs_id) REFERENCES gpkg_spatial_ref_sys(srs_id)
		)`,
		`CREATE TABLE IF NOT EXISTS gpkg_geometry_columns (
			table_name TEXT NOT NULL,
			column_name TEXT NOT NULL,
			geometry_type_name TEXT NOT NULL,
			srs_id INTEGER NOT NULL,
			z INTEGER NOT NULL,
			m INTEGER NOT NULL,
			CONSTRAINT pk_gc PRIMARY KEY (table_name, column_name),
			CONSTRAINT fk_gc_tn FOREIGN KEY (table_name) REFERENCES gpkg_contents(table_name),
			CONSTRAINT fk_gc_srs FOREIGN KEY (srs_id) REFERENCES gpkg_spatial_ref_sys(srs_id)
		)`,
	}

	for _, stmt := range stmts {
		_, err = db.ExecContext(ctx, stmt)
		if err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}

	// Insert default SRS entries required by GeoPackage spec.
	defaultSRS := []struct {
		name  string
		id    int
		org   string
		orgID int
		def   string
	}{
		{"Undefined cartesian SRS", -1, "NONE", -1, "undefined"},
		{"Undefined geographic SRS", 0, "NONE", 0, "undefined"},
		{"WGS 84 geodetic", 4326, "EPSG", 4326, `GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563]],PRIMEM["Greenwich",0],UNIT["degree",0.0174532925199433]]`},
	}

	for _, s := range defaultSRS {
		_, err = db.ExecContext(
			ctx,
			"INSERT OR IGNORE INTO gpkg_spatial_ref_sys (srs_name, srs_id, organization, organization_coordsys_id, definition) VALUES (?, ?, ?, ?, ?)",
			s.name, s.id, s.org, s.orgID, s.def,
		)
		if err != nil {
			return err
		}
	}

	// Insert the project CRS if it differs from defaults.
	if srsID != -1 && srsID != 0 && srsID != 4326 {
		_, err = db.ExecContext(
			ctx,
			"INSERT OR IGNORE INTO gpkg_spatial_ref_sys (srs_name, srs_id, organization, organization_coordsys_id, definition) VALUES (?, ?, ?, ?, ?)",
			crs, srsID, "EPSG", srsID, "undefined",
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func createReceiverTable(db *sql.DB, table results.ReceiverTable, srsID int) error {
	ctx := context.Background()

	// Build column definitions for indicator values.
	colDefs := make([]string, 0, len(table.IndicatorOrder)+5)
	colDefs = append(colDefs,
		"fid INTEGER PRIMARY KEY AUTOINCREMENT",
		"geom BLOB",
		"receiver_id TEXT NOT NULL",
		"x REAL NOT NULL",
		"y REAL NOT NULL",
		"height_m REAL NOT NULL",
	)

	for _, indicator := range table.IndicatorOrder {
		colName := sanitizeColumnName(indicator)
		colDefs = append(colDefs, colName+" REAL")
	}

	createSQL := "CREATE TABLE receivers (" + strings.Join(colDefs, ", ") + ")"

	_, err := db.ExecContext(ctx, createSQL)
	if err != nil {
		return fmt.Errorf("create receivers table: %w", err)
	}

	// Compute extent from records.
	minX, minY, maxX, maxY := computeReceiverExtent(table)

	// Register in gpkg_contents.
	_, err = db.ExecContext(
		ctx,
		`INSERT INTO gpkg_contents (table_name, data_type, identifier, description, last_change, min_x, min_y, max_x, max_y, srs_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"receivers", "features", "receivers", "Receiver points with noise indicators",
		time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		minX, minY, maxX, maxY, srsID,
	)
	if err != nil {
		return err
	}

	// Register geometry column.
	_, err = db.ExecContext(
		ctx,
		`INSERT INTO gpkg_geometry_columns (table_name, column_name, geometry_type_name, srs_id, z, m) VALUES (?, ?, ?, ?, ?, ?)`,
		"receivers", "geom", "POINT", srsID, 1, 0,
	)
	if err != nil {
		return err
	}

	return nil
}

func insertReceivers(db *sql.DB, table results.ReceiverTable) error {
	ctx := context.Background()

	// Build the INSERT statement.
	colNames := make([]string, 0, len(table.IndicatorOrder)+5)

	colNames = append(colNames, "geom", "receiver_id", "x", "y", "height_m")
	for _, indicator := range table.IndicatorOrder {
		colNames = append(colNames, sanitizeColumnName(indicator))
	}

	placeholders := make([]string, len(colNames))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	var sqlBuilder strings.Builder
	sqlBuilder.Grow(len(colNames)*16 + len(placeholders)*2 + 32)
	sqlBuilder.WriteString("INSERT INTO receivers (")
	sqlBuilder.WriteString(strings.Join(colNames, ", "))
	sqlBuilder.WriteString(") VALUES (")
	sqlBuilder.WriteString(strings.Join(placeholders, ", "))
	sqlBuilder.WriteString(")")
	insertSQL := sqlBuilder.String()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, record := range table.Records {
		geomBlob := encodeGPKGPoint(record.X, record.Y, record.HeightM, 0)

		args := make([]any, 0, len(colNames))
		args = append(args, geomBlob, record.ID, record.X, record.Y, record.HeightM)

		for _, indicator := range table.IndicatorOrder {
			args = append(args, record.Values[indicator])
		}

		_, err = stmt.ExecContext(ctx, args...)
		if err != nil {
			return fmt.Errorf("insert receiver %s: %w", record.ID, err)
		}
	}

	return tx.Commit()
}

func createContourTable(db *sql.DB, srsID int) error {
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `CREATE TABLE contours (
		fid INTEGER PRIMARY KEY AUTOINCREMENT,
		geom BLOB,
		level_db REAL NOT NULL,
		band_name TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO gpkg_contents (table_name, data_type, identifier, description, last_change, srs_id) VALUES (?, ?, ?, ?, ?, ?)`,
		"contours", "features", "contours", "Noise contour lines",
		time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), srsID,
	)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO gpkg_geometry_columns (table_name, column_name, geometry_type_name, srs_id, z, m) VALUES (?, ?, ?, ?, ?, ?)`,
		"contours", "geom", "LINESTRING", srsID, 0, 0,
	)
	if err != nil {
		return err
	}

	return nil
}

func insertContours(db *sql.DB, contours []ContourLine) error {
	ctx := context.Background()

	if len(contours) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO contours (geom, level_db, band_name) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range contours {
		if len(c.Points) < 2 {
			continue
		}

		geomBlob := encodeGPKGLineString(c.Points, 0)

		_, err = stmt.ExecContext(ctx, geomBlob, c.Level, c.BandName)
		if err != nil {
			return fmt.Errorf("insert contour level=%g: %w", c.Level, err)
		}
	}

	return tx.Commit()
}

// encodeGPKGPoint encodes a point as GeoPackage standard binary geometry (GP).
func encodeGPKGPoint(x float64, y float64, z float64, srsID int) []byte {
	// GeoPackage binary header (8 bytes) + WKB Point Z.
	// Header: magic (GP), version, flags, srs_id.
	// GeoPackage binary geometry format:
	// 2 bytes: magic = 0x4750 ('G','P')
	// 1 byte: version = 0
	// 1 byte: flags (bit 0: byte order 1=LE, bits 1-3: envelope type 0=none)
	// 4 bytes: srs_id
	// followed by WKB geometry

	// WKB PointZ: byte_order(1) + wkb_type(4) + x(8) + y(8) + z(8)
	total := 8 + 1 + 4 + 8 + 8 + 8
	buf := make([]byte, total)

	// GP header.
	buf[0] = 'G'
	buf[1] = 'P'
	buf[2] = 0    // version
	buf[3] = 0x01 // flags: little-endian, no envelope
	binary.LittleEndian.PutUint32(buf[4:], mustUint32(srsID))

	// WKB PointZ.
	offset := 8
	buf[offset] = 1 // little-endian
	offset++
	binary.LittleEndian.PutUint32(buf[offset:], mustUint32(1001)) // wkbPointZ
	offset += 4
	binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(x))
	offset += 8
	binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(y))
	offset += 8
	binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(z))

	return buf
}

// encodeGPKGLineString encodes a linestring as GeoPackage standard binary geometry.
func encodeGPKGLineString(points [][2]float64, srsID int) []byte {
	// GP header(8) + WKB byte_order(1) + type(4) + numPoints(4) + points(numPoints*16)
	total := 8 + 1 + 4 + 4 + len(points)*16
	buf := make([]byte, total)

	buf[0] = 'G'
	buf[1] = 'P'
	buf[2] = 0
	buf[3] = 0x01
	binary.LittleEndian.PutUint32(buf[4:], mustUint32(srsID))

	offset := 8
	buf[offset] = 1 // little-endian
	offset++
	binary.LittleEndian.PutUint32(buf[offset:], mustUint32(2)) // wkbLineString
	offset += 4
	binary.LittleEndian.PutUint32(buf[offset:], mustUint32(len(points)))
	offset += 4

	for _, pt := range points {
		binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(pt[0]))
		offset += 8
		binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(pt[1]))
		offset += 8
	}

	return buf
}

func computeReceiverExtent(table results.ReceiverTable) (float64, float64, float64, float64) {
	if len(table.Records) == 0 {
		return 0, 0, 0, 0
	}

	minX, minY := table.Records[0].X, table.Records[0].Y
	maxX, maxY := minX, minY

	for _, r := range table.Records[1:] {
		if r.X < minX {
			minX = r.X
		}

		if r.X > maxX {
			maxX = r.X
		}

		if r.Y < minY {
			minY = r.Y
		}

		if r.Y > maxY {
			maxY = r.Y
		}
	}

	return minX, minY, maxX, maxY
}

// ModelFeature is a simplified representation of a model feature for GeoPackage export.
// This avoids importing modelgeojson in the export package.
type ModelFeature struct {
	ID           string
	Kind         string  // source, building, barrier, receiver
	SourceType   string  // point, line, area (for sources)
	HeightM      float64 // 0 if not applicable
	GeometryType string  // Point, LineString, Polygon
	Coordinates  any     // parsed GeoJSON coordinates
}

// ExportModelFeaturesGeoPackage writes model features (sources, buildings, barriers)
// as an OGC GeoPackage with mixed geometry types.
func ExportModelFeaturesGeoPackage(path string, features []ModelFeature, crs string, srsID int) error {
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return fmt.Errorf("create gpkg directory: %w", err)
	}

	_ = os.Remove(path)

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open gpkg database: %w", err)
	}
	defer db.Close()

	err = initGeoPackage(db, crs, srsID)
	if err != nil {
		return fmt.Errorf("init geopackage: %w", err)
	}

	err = createModelFeaturesTable(db, features, srsID)
	if err != nil {
		return fmt.Errorf("create model_features table: %w", err)
	}

	err = insertModelFeatures(db, features, srsID)
	if err != nil {
		return fmt.Errorf("insert model features: %w", err)
	}

	return nil
}

func createModelFeaturesTable(db *sql.DB, features []ModelFeature, srsID int) error {
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `CREATE TABLE model_features (
		fid INTEGER PRIMARY KEY AUTOINCREMENT,
		geom BLOB,
		feature_id TEXT,
		kind TEXT,
		source_type TEXT,
		height_m REAL
	)`)
	if err != nil {
		return err
	}

	minX, minY, maxX, maxY := computeModelFeaturesExtent(features)

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO gpkg_contents (table_name, data_type, identifier, description, last_change, min_x, min_y, max_x, max_y, srs_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"model_features", "features", "model_features", "Model features (sources, buildings, barriers)",
		time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		minX, minY, maxX, maxY, srsID,
	)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO gpkg_geometry_columns (table_name, column_name, geometry_type_name, srs_id, z, m) VALUES (?, ?, ?, ?, ?, ?)`,
		"model_features", "geom", "GEOMETRY", srsID, 0, 0,
	)
	if err != nil {
		return err
	}

	return nil
}

func insertModelFeatures(db *sql.DB, features []ModelFeature, srsID int) error {
	ctx := context.Background()

	if len(features) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.PrepareContext(ctx, "INSERT INTO model_features (geom, feature_id, kind, source_type, height_m) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, f := range features {
		geomBlob, err := parseModelGeometry(f.GeometryType, f.Coordinates, srsID)
		if err != nil {
			return fmt.Errorf("encode geometry for feature %s: %w", f.ID, err)
		}

		_, err = stmt.ExecContext(ctx, geomBlob, f.ID, f.Kind, f.SourceType, f.HeightM)
		if err != nil {
			return fmt.Errorf("insert feature %s: %w", f.ID, err)
		}
	}

	return tx.Commit()
}

func parseModelGeometry(geomType string, coords any, srsID int) ([]byte, error) {
	switch geomType {
	case "Point":
		return parsePointCoords(coords, srsID)
	case "LineString":
		return parseLineStringCoords(coords, srsID)
	case "Polygon":
		return parsePolygonCoords(coords, srsID)
	default:
		return nil, fmt.Errorf("unsupported geometry type: %s", geomType)
	}
}

func parsePointCoords(coords any, srsID int) ([]byte, error) {
	arr, ok := coords.([]any)
	if !ok {
		return nil, fmt.Errorf("point coordinates: expected []any, got %T", coords)
	}

	if len(arr) < 2 {
		return nil, fmt.Errorf("point coordinates: need at least 2 values, got %d", len(arr))
	}

	x, err := toFloat64(arr[0])
	if err != nil {
		return nil, fmt.Errorf("point x: %w", err)
	}

	y, err := toFloat64(arr[1])
	if err != nil {
		return nil, fmt.Errorf("point y: %w", err)
	}

	return encodeGPKGPoint2D(x, y, srsID), nil
}

func parseLineStringCoords(coords any, srsID int) ([]byte, error) {
	arr, ok := coords.([]any)
	if !ok {
		return nil, fmt.Errorf("linestring coordinates: expected []any, got %T", coords)
	}

	points := make([][2]float64, 0, len(arr))

	for i, pt := range arr {
		pair, ok := pt.([]any)
		if !ok {
			return nil, fmt.Errorf("linestring point %d: expected []any, got %T", i, pt)
		}

		if len(pair) < 2 {
			return nil, fmt.Errorf("linestring point %d: need at least 2 values", i)
		}

		x, err := toFloat64(pair[0])
		if err != nil {
			return nil, fmt.Errorf("linestring point %d x: %w", i, err)
		}

		y, err := toFloat64(pair[1])
		if err != nil {
			return nil, fmt.Errorf("linestring point %d y: %w", i, err)
		}

		points = append(points, [2]float64{x, y})
	}

	return encodeGPKGLineString(points, srsID), nil
}

func parsePolygonCoords(coords any, srsID int) ([]byte, error) {
	arr, ok := coords.([]any)
	if !ok {
		return nil, fmt.Errorf("polygon coordinates: expected []any, got %T", coords)
	}

	rings := make([][][2]float64, 0, len(arr))

	for i, ringRaw := range arr {
		ringArr, ok := ringRaw.([]any)
		if !ok {
			return nil, fmt.Errorf("polygon ring %d: expected []any, got %T", i, ringRaw)
		}

		ring := make([][2]float64, 0, len(ringArr))

		for j, pt := range ringArr {
			pair, ok := pt.([]any)
			if !ok {
				return nil, fmt.Errorf("polygon ring %d point %d: expected []any, got %T", i, j, pt)
			}

			if len(pair) < 2 {
				return nil, fmt.Errorf("polygon ring %d point %d: need at least 2 values", i, j)
			}

			x, err := toFloat64(pair[0])
			if err != nil {
				return nil, fmt.Errorf("polygon ring %d point %d x: %w", i, j, err)
			}

			y, err := toFloat64(pair[1])
			if err != nil {
				return nil, fmt.Errorf("polygon ring %d point %d y: %w", i, j, err)
			}

			ring = append(ring, [2]float64{x, y})
		}

		rings = append(rings, ring)
	}

	return encodeGPKGPolygon(rings, srsID), nil
}

func toFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case json.Number:
		return val.Float64()
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

// encodeGPKGPoint2D encodes a 2D point as GeoPackage standard binary geometry.
func encodeGPKGPoint2D(x float64, y float64, srsID int) []byte {
	// GP header(8) + WKB byte_order(1) + type(4) + x(8) + y(8)
	total := 8 + 1 + 4 + 8 + 8
	buf := make([]byte, total)

	buf[0] = 'G'
	buf[1] = 'P'
	buf[2] = 0
	buf[3] = 0x01
	binary.LittleEndian.PutUint32(buf[4:], mustUint32(srsID))

	offset := 8
	buf[offset] = 1 // little-endian
	offset++
	binary.LittleEndian.PutUint32(buf[offset:], mustUint32(1)) // wkbPoint
	offset += 4
	binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(x))
	offset += 8
	binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(y))

	return buf
}

// encodeGPKGPolygon encodes a polygon as GeoPackage standard binary geometry.
func encodeGPKGPolygon(rings [][][2]float64, srsID int) []byte {
	// GP header(8) + WKB byte_order(1) + type(4) + numRings(4)
	// + for each ring: numPoints(4) + points(numPoints*16)
	total := 8 + 1 + 4 + 4
	for _, ring := range rings {
		total += 4 + len(ring)*16
	}

	buf := make([]byte, total)

	buf[0] = 'G'
	buf[1] = 'P'
	buf[2] = 0
	buf[3] = 0x01
	binary.LittleEndian.PutUint32(buf[4:], mustUint32(srsID))

	offset := 8
	buf[offset] = 1 // little-endian
	offset++
	binary.LittleEndian.PutUint32(buf[offset:], mustUint32(3)) // wkbPolygon
	offset += 4
	binary.LittleEndian.PutUint32(buf[offset:], mustUint32(len(rings)))
	offset += 4

	for _, ring := range rings {
		binary.LittleEndian.PutUint32(buf[offset:], mustUint32(len(ring)))
		offset += 4

		for _, pt := range ring {
			binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(pt[0]))
			offset += 8
			binary.LittleEndian.PutUint64(buf[offset:], math.Float64bits(pt[1]))
			offset += 8
		}
	}

	return buf
}

func computeModelFeaturesExtent(features []ModelFeature) (float64, float64, float64, float64) {
	if len(features) == 0 {
		return 0, 0, 0, 0
	}

	minX := math.MaxFloat64
	minY := math.MaxFloat64
	maxX := -math.MaxFloat64
	maxY := -math.MaxFloat64

	collectPoint := func(x, y float64) {
		if x < minX {
			minX = x
		}

		if x > maxX {
			maxX = x
		}

		if y < minY {
			minY = y
		}

		if y > maxY {
			maxY = y
		}
	}

	found := false

	for _, f := range features {
		collectFromCoords(f.Coordinates, collectPoint, &found)
	}

	if !found {
		return 0, 0, 0, 0
	}

	return minX, minY, maxX, maxY
}

func collectFromCoords(coords any, collect func(x, y float64), found *bool) {
	arr, ok := coords.([]any)
	if !ok {
		return
	}

	if len(arr) == 0 {
		return
	}

	// Check if this is a coordinate pair (leaf): first element is a number.
	_, err := toFloat64(arr[0])
	if err == nil && len(arr) >= 2 {
		x, xerr := toFloat64(arr[0])
		y, yerr := toFloat64(arr[1])

		if xerr == nil && yerr == nil {
			collect(x, y)

			*found = true

			return
		}
	}

	// Otherwise recurse into nested arrays.
	for _, elem := range arr {
		collectFromCoords(elem, collect, found)
	}
}

func sanitizeColumnName(name string) string {
	// Replace non-alphanumeric characters with underscores.
	var b strings.Builder

	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}

	result := b.String()
	if result == "" {
		return "value"
	}

	// SQL column names cannot start with a digit.
	if result[0] >= '0' && result[0] <= '9' {
		result = "v_" + result
	}

	return result
}
