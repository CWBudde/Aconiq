// Package gpkgimport reads GeoPackage (.gpkg) files and converts feature
// layers into the project model format.
package gpkgimport

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	_ "modernc.org/sqlite" // register sqlite driver
)

// maxIdentifierLength bounds a layer name before it is embedded in a query.
// SQLite has no practical identifier length limit, but no real GeoPackage
// layer name approaches this.
const maxIdentifierLength = 255

// openReadOnly opens a GeoPackage for reading only.
//
// The file is untrusted third-party input, so the handle must not be able to
// modify it: the default DSN would open READWRITE|CREATE. "mode=ro" downgrades
// the connection to read-only and "immutable=1" additionally tells SQLite the
// file cannot change, which disables locking and ignores any -wal/-shm
// sidecars a hostile file might point at.
func openReadOnly(path string) (*sql.DB, error) {
	dsn, err := readOnlyDSN(path)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("gpkg: open %q: %w", path, err)
	}

	return db, nil
}

// readOnlyDSN builds a SQLite URI DSN for path. The path is percent-encoded so
// that a file name containing "?" or "#" cannot inject URI parameters.
func readOnlyDSN(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("gpkg: resolve %q: %w", path, err)
	}

	slashed := filepath.ToSlash(abs)
	if !strings.HasPrefix(slashed, "/") {
		// Windows drive-letter paths need a leading slash to keep the drive
		// letter out of the URI authority.
		slashed = "/" + slashed
	}

	u := url.URL{
		Scheme:   "file",
		Path:     slashed,
		RawQuery: "mode=ro&immutable=1",
	}

	return u.String(), nil
}

// quoteIdentifier validates and quotes a table name for interpolation into a
// query.
//
// Layer names come from gpkg_contents and gpkg_geometry_columns, which are
// tables *inside the untrusted file*, so they are attacker-controlled. The
// allowlist below admits the characters that appear in real layer names
// (including non-ASCII letters, spaces, dots and dashes) and rejects
// everything a statement could be broken out with: quotes, brackets,
// semicolons, whitespace other than a plain space, and control characters.
// The result is additionally double-quoted with embedded quotes doubled, so
// even an allowlisted name cannot terminate the identifier early.
func quoteIdentifier(name string) (string, error) {
	if name == "" || len(name) > maxIdentifierLength {
		return "", fmt.Errorf("gpkg: invalid table name %q", name)
	}

	for i, r := range name {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return "", fmt.Errorf("gpkg: refusing table name %q: must start with a letter or underscore", name)
			}

			continue
		}

		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' || r == ' ' {
			continue
		}

		return "", fmt.Errorf("gpkg: refusing table name %q: invalid character %q", name, r)
	}

	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`, nil
}

// LayerInfo describes one feature layer in a GeoPackage.
type LayerInfo struct {
	Name        string
	Description string
	GeomColumn  string
	IDColumn    string // typically "fid" or "id"
}

// ListLayers returns all feature layers in the GeoPackage.
func ListLayers(path string) ([]LayerInfo, error) {
	db, err := openReadOnly(path)
	if err != nil {
		return nil, err
	}

	defer db.Close()

	ctx := context.Background()

	rows, err := db.QueryContext(ctx, `SELECT table_name, description FROM gpkg_contents WHERE data_type = 'features'`)
	if err != nil {
		return nil, fmt.Errorf("gpkg: query gpkg_contents: %w", err)
	}

	defer rows.Close()

	var layers []LayerInfo

	for rows.Next() {
		var name, description string

		scanErr := rows.Scan(&name, &description)
		if scanErr != nil {
			return nil, fmt.Errorf("gpkg: scan gpkg_contents row: %w", scanErr)
		}

		layers = append(layers, LayerInfo{
			Name:        name,
			Description: description,
		})
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, fmt.Errorf("gpkg: iterate gpkg_contents: %w", rowsErr)
	}

	return layers, nil
}

// ReadResult holds the result of reading a GeoPackage layer.
type ReadResult struct {
	Collection modelgeojson.FeatureCollection
	EPSGCode   int // 0 if CRS could not be determined
}

// ReadLayer reads features from a named layer and returns a GeoJSON-compatible
// FeatureCollection ready for Normalize.
func ReadLayer(path string, layerName string) (modelgeojson.FeatureCollection, error) {
	result, err := ReadLayerWithCRS(path, layerName)
	if err != nil {
		return modelgeojson.FeatureCollection{}, err
	}

	return result.Collection, nil
}

// ReadLayerWithCRS reads features from a named layer and also extracts the CRS
// from the GeoPackage spatial_ref_sys table.
func ReadLayerWithCRS(path string, layerName string) (ReadResult, error) {
	quotedTable, err := quoteIdentifier(layerName)
	if err != nil {
		return ReadResult{}, err
	}

	db, err := openReadOnly(path)
	if err != nil {
		return ReadResult{}, err
	}

	defer db.Close()

	ctx := context.Background()

	geomCol, err := queryGeomColumn(ctx, db, layerName)
	if err != nil {
		return ReadResult{}, err
	}

	epsg := querySRSID(ctx, db, layerName)

	colNames, err := queryColumnNames(ctx, db, layerName, quotedTable)
	if err != nil {
		return ReadResult{}, err
	}

	features, err := queryFeatures(ctx, db, layerName, quotedTable, colNames, geomCol)
	if err != nil {
		return ReadResult{}, err
	}

	return ReadResult{
		Collection: modelgeojson.FeatureCollection{
			Type:     modelgeojson.TypeFeatureCollection,
			Features: features,
		},
		EPSGCode: epsg,
	}, nil
}

// querySRSID extracts the EPSG code for a layer from gpkg_geometry_columns.
// Returns 0 if the SRS cannot be determined.
func querySRSID(ctx context.Context, db *sql.DB, tableName string) int {
	var srsID int

	err := db.QueryRowContext(
		ctx,
		`SELECT srs_id FROM gpkg_geometry_columns WHERE table_name = ?`,
		tableName,
	).Scan(&srsID)
	if err != nil {
		return 0
	}

	return srsID
}

func queryGeomColumn(ctx context.Context, db *sql.DB, tableName string) (string, error) {
	var geomCol string

	err := db.QueryRowContext(
		ctx,
		`SELECT column_name FROM gpkg_geometry_columns WHERE table_name = ?`,
		tableName,
	).Scan(&geomCol)
	if err != nil {
		return "", fmt.Errorf("gpkg: find geometry column for %q: %w", tableName, err)
	}

	return geomCol, nil
}

// queryColumnNames retrieves the column names for a layer by querying one row with LIMIT 0.
func queryColumnNames(ctx context.Context, db *sql.DB, tableName, quotedTable string) ([]string, error) {
	//nolint:gosec,unqueryvet // quotedTable passed through quoteIdentifier; SELECT * needed to discover the dynamic column list
	rows, err := db.QueryContext(ctx, "SELECT * FROM "+quotedTable+" LIMIT 0")
	if err != nil {
		return nil, fmt.Errorf("gpkg: get column names for %q: %w", tableName, err)
	}

	defer rows.Close()

	names, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("gpkg: read column names for %q: %w", tableName, err)
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, fmt.Errorf("gpkg: rows error for %q: %w", tableName, rowsErr)
	}

	return names, nil
}

// queryFeatures scans all features from a table.
func queryFeatures(ctx context.Context, db *sql.DB, tableName, quotedTable string, colNames []string, geomCol string) ([]modelgeojson.GeoJSONFeature, error) {
	//nolint:gosec,unqueryvet // quotedTable passed through quoteIdentifier; SELECT * needed for the dynamic column scan
	rows, err := db.QueryContext(ctx, "SELECT * FROM "+quotedTable)
	if err != nil {
		return nil, fmt.Errorf("gpkg: query layer %q: %w", tableName, err)
	}

	defer rows.Close()

	var features []modelgeojson.GeoJSONFeature

	for rows.Next() {
		feature, scanErr := scanFeature(rows, colNames, geomCol)
		if scanErr != nil {
			return nil, scanErr
		}

		if feature != nil {
			features = append(features, *feature)
		}
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, fmt.Errorf("gpkg: iterate layer %q: %w", tableName, rowsErr)
	}

	return features, nil
}

func scanFeature(rows *sql.Rows, colNames []string, geomCol string) (*modelgeojson.GeoJSONFeature, error) {
	values := make([]any, len(colNames))
	ptrs := make([]any, len(colNames))

	for i := range values {
		ptrs[i] = &values[i]
	}

	scanErr := rows.Scan(ptrs...)
	if scanErr != nil {
		return nil, fmt.Errorf("gpkg: scan row: %w", scanErr)
	}

	var (
		geomType  string
		coords    any
		featureID any
	)

	props := make(map[string]any)

	for i, col := range colNames {
		val := values[i]

		if col == geomCol {
			blob, ok := val.([]byte)
			if !ok || len(blob) == 0 {
				return nil, nil //nolint:nilnil // nil feature signals "skip row"
			}

			gt, c, err := DecodeGPKGBlob(blob)
			if err != nil {
				return nil, fmt.Errorf("gpkg: decode geometry for column %q: %w", col, err)
			}

			if gt == "" {
				return nil, nil //nolint:nilnil // nil feature signals "skip row" (empty geometry)
			}

			geomType = gt
			coords = c

			continue
		}

		if col == "fid" || col == "id" {
			featureID = formatID(val)
		}

		props[col] = normalizeValue(val)
	}

	if geomType == "" {
		return nil, nil //nolint:nilnil // nil feature signals "skip row" (no geometry column found)
	}

	return &modelgeojson.GeoJSONFeature{
		Type:       "Feature",
		ID:         featureID,
		Properties: props,
		Geometry: modelgeojson.Geometry{
			Type:        geomType,
			Coordinates: coords,
		},
	}, nil
}

func formatID(val any) string {
	if val == nil {
		return ""
	}

	switch v := val.(type) {
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func normalizeValue(val any) any {
	if val == nil {
		return nil
	}

	if b, ok := val.([]byte); ok {
		return string(b)
	}

	return val
}
