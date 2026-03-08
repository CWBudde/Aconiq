// Package gpkgimport reads GeoPackage (.gpkg) files and converts feature
// layers into the project model format.
package gpkgimport

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	_ "modernc.org/sqlite" // register sqlite driver
)

// LayerInfo describes one feature layer in a GeoPackage.
type LayerInfo struct {
	Name        string
	Description string
	GeomColumn  string
	IDColumn    string // typically "fid" or "id"
}

// ListLayers returns all feature layers in the GeoPackage.
func ListLayers(path string) ([]LayerInfo, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("gpkg: open %q: %w", path, err)
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

// ReadLayer reads features from a named layer and returns a GeoJSON-compatible
// FeatureCollection ready for Normalize.
func ReadLayer(path string, layerName string) (modelgeojson.FeatureCollection, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return modelgeojson.FeatureCollection{}, fmt.Errorf("gpkg: open %q: %w", path, err)
	}

	defer db.Close()

	ctx := context.Background()

	geomCol, err := queryGeomColumn(ctx, db, layerName)
	if err != nil {
		return modelgeojson.FeatureCollection{}, err
	}

	colNames, err := queryColumnNames(ctx, db, layerName)
	if err != nil {
		return modelgeojson.FeatureCollection{}, err
	}

	features, err := queryFeatures(ctx, db, layerName, colNames, geomCol)
	if err != nil {
		return modelgeojson.FeatureCollection{}, err
	}

	return modelgeojson.FeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
	}, nil
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
func queryColumnNames(ctx context.Context, db *sql.DB, tableName string) ([]string, error) {
	//nolint:gosec,unqueryvet // table name comes from gpkg_geometry_columns metadata; SELECT * needed to discover dynamic column list
	rows, err := db.QueryContext(ctx, "SELECT * FROM "+tableName+" LIMIT 0")
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
func queryFeatures(ctx context.Context, db *sql.DB, tableName string, colNames []string, geomCol string) ([]modelgeojson.GeoJSONFeature, error) {
	//nolint:gosec,unqueryvet // table name comes from gpkg_geometry_columns metadata; SELECT * needed for dynamic column scan
	rows, err := db.QueryContext(ctx, "SELECT * FROM "+tableName)
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

	var geomType string
	var coords any
	var featureID any
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
