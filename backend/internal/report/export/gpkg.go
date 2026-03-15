package export

import (
	"database/sql"
	"encoding/binary"
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
	if err := table.Validate(); err != nil {
		return fmt.Errorf("validate receiver table: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create gpkg directory: %w", err)
	}

	// Remove existing file to start fresh.
	_ = os.Remove(path)

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open gpkg database: %w", err)
	}
	defer db.Close()

	if err = initGeoPackage(db, crs, srsID); err != nil {
		return fmt.Errorf("init geopackage: %w", err)
	}

	if err = createReceiverTable(db, table, srsID); err != nil {
		return fmt.Errorf("create receiver table: %w", err)
	}

	if err = insertReceivers(db, table); err != nil {
		return fmt.Errorf("insert receivers: %w", err)
	}

	return nil
}

// ExportContourGeoPackage writes contour lines as an OGC GeoPackage.
func ExportContourGeoPackage(path string, contours []ContourLine, crs string, srsID int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create gpkg directory: %w", err)
	}

	_ = os.Remove(path)

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open gpkg database: %w", err)
	}
	defer db.Close()

	if err = initGeoPackage(db, crs, srsID); err != nil {
		return fmt.Errorf("init geopackage: %w", err)
	}

	if err = createContourTable(db, srsID); err != nil {
		return fmt.Errorf("create contour table: %w", err)
	}

	if err = insertContours(db, contours); err != nil {
		return fmt.Errorf("insert contours: %w", err)
	}

	return nil
}

func initGeoPackage(db *sql.DB, crs string, srsID int) error {
	// Set GeoPackage application_id.
	_, err := db.Exec("PRAGMA application_id = 0x47504B47") // 'GPKG'
	if err != nil {
		return err
	}

	_, err = db.Exec("PRAGMA user_version = 10301") // GeoPackage 1.3.1
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
		_, err = db.Exec(stmt)
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
		_, err = db.Exec(
			"INSERT OR IGNORE INTO gpkg_spatial_ref_sys (srs_name, srs_id, organization, organization_coordsys_id, definition) VALUES (?, ?, ?, ?, ?)",
			s.name, s.id, s.org, s.orgID, s.def,
		)
		if err != nil {
			return err
		}
	}

	// Insert the project CRS if it differs from defaults.
	if srsID != -1 && srsID != 0 && srsID != 4326 {
		_, err = db.Exec(
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
	// Build column definitions for indicator values.
	colDefs := []string{
		"fid INTEGER PRIMARY KEY AUTOINCREMENT",
		"geom BLOB",
		"receiver_id TEXT NOT NULL",
		"x REAL NOT NULL",
		"y REAL NOT NULL",
		"height_m REAL NOT NULL",
	}

	for _, indicator := range table.IndicatorOrder {
		colName := sanitizeColumnName(indicator)
		colDefs = append(colDefs, fmt.Sprintf("%s REAL", colName))
	}

	createSQL := fmt.Sprintf("CREATE TABLE receivers (%s)", strings.Join(colDefs, ", "))

	_, err := db.Exec(createSQL)
	if err != nil {
		return fmt.Errorf("create receivers table: %w", err)
	}

	// Compute extent from records.
	minX, minY, maxX, maxY := computeReceiverExtent(table)

	// Register in gpkg_contents.
	_, err = db.Exec(
		`INSERT INTO gpkg_contents (table_name, data_type, identifier, description, last_change, min_x, min_y, max_x, max_y, srs_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"receivers", "features", "receivers", "Receiver points with noise indicators",
		time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		minX, minY, maxX, maxY, srsID,
	)
	if err != nil {
		return err
	}

	// Register geometry column.
	_, err = db.Exec(
		`INSERT INTO gpkg_geometry_columns (table_name, column_name, geometry_type_name, srs_id, z, m) VALUES (?, ?, ?, ?, ?, ?)`,
		"receivers", "geom", "POINT", srsID, 1, 0,
	)
	if err != nil {
		return err
	}

	return nil
}

func insertReceivers(db *sql.DB, table results.ReceiverTable) error {
	// Build the INSERT statement.
	colNames := []string{"geom", "receiver_id", "x", "y", "height_m"}
	for _, indicator := range table.IndicatorOrder {
		colNames = append(colNames, sanitizeColumnName(indicator))
	}

	placeholders := make([]string, len(colNames))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	insertSQL := fmt.Sprintf("INSERT INTO receivers (%s) VALUES (%s)",
		strings.Join(colNames, ", "),
		strings.Join(placeholders, ", "),
	)

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.Prepare(insertSQL)
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

		_, err = stmt.Exec(args...)
		if err != nil {
			return fmt.Errorf("insert receiver %s: %w", record.ID, err)
		}
	}

	return tx.Commit()
}

func createContourTable(db *sql.DB, srsID int) error {
	_, err := db.Exec(`CREATE TABLE contours (
		fid INTEGER PRIMARY KEY AUTOINCREMENT,
		geom BLOB,
		level_db REAL NOT NULL,
		band_name TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(
		`INSERT INTO gpkg_contents (table_name, data_type, identifier, description, last_change, srs_id) VALUES (?, ?, ?, ?, ?, ?)`,
		"contours", "features", "contours", "Noise contour lines",
		time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), srsID,
	)
	if err != nil {
		return err
	}

	_, err = db.Exec(
		`INSERT INTO gpkg_geometry_columns (table_name, column_name, geometry_type_name, srs_id, z, m) VALUES (?, ?, ?, ?, ?, ?)`,
		"contours", "geom", "LINESTRING", srsID, 0, 0,
	)
	if err != nil {
		return err
	}

	return nil
}

func insertContours(db *sql.DB, contours []ContourLine) error {
	if len(contours) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.Prepare("INSERT INTO contours (geom, level_db, band_name) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range contours {
		if len(c.Points) < 2 {
			continue
		}

		geomBlob := encodeGPKGLineString(c.Points, 0)

		_, err = stmt.Exec(geomBlob, c.Level, c.BandName)
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
	buf := make([]byte, 8+8+8+8+4) // header(8) + WKB type(4+1) ... actually let me compute

	// GeoPackage binary geometry format:
	// 2 bytes: magic = 0x4750 ('G','P')
	// 1 byte: version = 0
	// 1 byte: flags (bit 0: byte order 1=LE, bits 1-3: envelope type 0=none)
	// 4 bytes: srs_id
	// followed by WKB geometry

	// WKB PointZ: byte_order(1) + wkb_type(4) + x(8) + y(8) + z(8)
	total := 8 + 1 + 4 + 8 + 8 + 8
	buf = make([]byte, total)

	// GP header.
	buf[0] = 'G'
	buf[1] = 'P'
	buf[2] = 0    // version
	buf[3] = 0x01 // flags: little-endian, no envelope
	binary.LittleEndian.PutUint32(buf[4:], uint32(srsID))

	// WKB PointZ.
	offset := 8
	buf[offset] = 1 // little-endian
	offset++
	binary.LittleEndian.PutUint32(buf[offset:], 1001) // wkbPointZ
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
	binary.LittleEndian.PutUint32(buf[4:], uint32(srsID))

	offset := 8
	buf[offset] = 1 // little-endian
	offset++
	binary.LittleEndian.PutUint32(buf[offset:], 2) // wkbLineString
	offset += 4
	binary.LittleEndian.PutUint32(buf[offset:], uint32(len(points)))
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
