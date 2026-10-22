package export

// The SoundPLAN hand-off: the project model as ESRI Shapefiles, one per object
// type, plus a mapping report that says which column holds which model
// property and what could not be carried. See docs/research/soundplan-export.md
// for why Shapefile, and for the manual import steps on the SoundPLAN side.
//
// This exports model input, never results. SoundPLAN computes the levels; the
// only claim here is that the geometry and the attributes arrive as the model
// holds them, or are listed in the report as not arriving.

import (
	"bufio"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aconiq/backend/internal/atomicfile"
	"github.com/aconiq/backend/internal/jsonio"
)

// SoundPLANOptions carries what the mapping needs beyond the features.
type SoundPLANOptions struct {
	// CRS is the CRS the features are in, and the one the files are labelled
	// with. It must be one ESRIWKTForEPSG knows: SoundPLAN computes distances
	// in the plane of the coordinates it is given.
	CRS string
	// SourceCRS is the CRS the model is stored in, when the caller had to
	// project it into CRS first; empty when it did not.
	SourceCRS string
	// Date is stamped into every .dbf header.
	Date time.Time
}

// SoundPLANReport is soundplan-mapping.json: what a SoundPLAN user needs to
// fill the import assignment table, and what did not survive the export.
type SoundPLANReport struct {
	Format             string                  `json:"format"`
	Purpose            string                  `json:"purpose"`
	CRS                string                  `json:"crs"`
	SourceCRS          string                  `json:"source_crs,omitempty"`
	ElevationReference string                  `json:"elevation_reference"`
	Layers             []SoundPLANLayerReport  `json:"layers"`
	Terrain            *SoundPLANTerrainReport `json:"terrain,omitempty"`
	Lossy              []SoundPLANLossyEntry   `json:"lossy"`
}

// SoundPLANLayerReport describes one written Shapefile.
type SoundPLANLayerReport struct {
	Layer    string            `json:"layer"`
	File     string            `json:"file"`
	Geometry string            `json:"geometry"`
	Features int               `json:"features"`
	Records  int               `json:"records"`
	Columns  []SoundPLANColumn `json:"columns"`
}

// SoundPLANColumn is one .dbf field and the model property it holds.
type SoundPLANColumn struct {
	Field    string `json:"field"`
	Property string `json:"property"`
	Type     string `json:"type"`
	Decimals *int   `json:"decimals,omitempty"`
	Unit     string `json:"unit,omitempty"`
}

// SoundPLANTerrainReport describes the elevation-point file.
type SoundPLANTerrainReport struct {
	File     string `json:"file"`
	Points   int    `json:"points"`
	Layout   string `json:"layout"`
	Decimals int    `json:"decimals"`
}

// SoundPLANLossyEntry is one thing the export could not carry.
type SoundPLANLossyEntry struct {
	FeatureID string `json:"feature_id"`
	Property  string `json:"property"`
	Reason    string `json:"reason"`
}

// SoundPLANReportFile and SoundPLANTerrainFile are the names written next to
// the layers.
const (
	SoundPLANReportFile  = "soundplan-mapping.json"
	SoundPLANTerrainFile = "terrain.txt"
)

// soundPLANTerrainDecimals is the rounding boundary of terrain.txt: millimetres.
const soundPLANTerrainDecimals = 3

// passthroughMaxDecimals bounds the fractional digits of a property the
// mapping has no declared unit for. A value that needs more is rounded there,
// and the rounding is reported.
const passthroughMaxDecimals = 6

type soundPLANLayerSpec struct {
	name       string
	kind       string
	sourceType string
	shape      ShapeType
}

// soundPLANLayers is the layer split, in the order the files are written and
// reported. One Shapefile holds one geometry type, and SoundPLAN imports one
// object type per assignment, so the split is by kind and, for sources, by
// source type.
var soundPLANLayers = []soundPLANLayerSpec{
	{name: "buildings", kind: "building", shape: ShapePolygon},
	{name: "barriers", kind: "barrier", shape: ShapePolyLine},
	{name: "receivers", kind: "receiver", shape: ShapePoint},
	{name: "sources_point", kind: kindSource, sourceType: "point", shape: ShapePoint},
	{name: "sources_line", kind: kindSource, sourceType: "line", shape: ShapePolyLine},
	{name: "sources_area", kind: kindSource, sourceType: "area", shape: ShapePolygon},
	{name: "calc_area", kind: "calc-area", shape: ShapePolygon},
	{name: "ground_zones", kind: "ground-zone", shape: ShapePolygon},
}

type coreColumn struct {
	property string
	field    string
	kind     FieldKind
	decimals int
	unit     string
}

// soundPLANCoreColumns are the properties with a fixed field name, declared
// unit and declared rounding. Everything else passes through under an
// allocated name. A core column is written on a layer only when one of its
// features carries the property.
var soundPLANCoreColumns = []coreColumn{
	{property: "height_m", field: "HEIGHT_M", kind: FieldNumeric, decimals: 3, unit: "m"},
	{property: "iso9613_sound_power_level_db", field: "LW_DB", kind: FieldNumeric, decimals: 2, unit: "dB"},
	{property: "iso9613_source_height_m", field: "HS_M", kind: FieldNumeric, decimals: 3, unit: "m"},
	{property: "iso9613_directivity_correction_db", field: "K_DIR_DB", kind: FieldNumeric, decimals: 2, unit: "dB"},
	{property: "iso9613_tonality_correction_db", field: "K_TON_DB", kind: FieldNumeric, decimals: 2, unit: "dB"},
	{property: "iso9613_impulsivity_correction_db", field: "K_IMP_DB", kind: FieldNumeric, decimals: 2, unit: "dB"},
	{property: "ground_factor", field: "G", kind: FieldNumeric, decimals: 3, unit: "1"},
	{property: "bimschv16_area_category", field: "AREA_CAT", kind: FieldChar},
}

// Properties the layer itself expresses, so they are neither columns nor loss.
var soundPLANImpliedProperties = map[string]bool{"id": true, "kind": true, "source_type": true}

// The GeoJSON geometry types and model vocabulary the mapping switches on.
const (
	geoJSONPoint           = "Point"
	geoJSONMultiPoint      = "MultiPoint"
	geoJSONLineString      = "LineString"
	geoJSONMultiLineString = "MultiLineString"
	geoJSONPolygon         = "Polygon"
	geoJSONMultiPolygon    = "MultiPolygon"
	kindSource             = "source"
	geometryProperty       = "geometry"
)

const (
	partField    = "PART"
	partProperty = "(multipoint part)"
	idField      = "ID"
)

// ExportSoundPLANShapefiles writes one Shapefile per non-empty layer into dir
// and returns the report with the paths it wrote. The report is returned, not
// written: the caller adds the terrain to it first, then calls Write.
func ExportSoundPLANShapefiles(dir string, features []ModelFeature, opts SoundPLANOptions) (SoundPLANReport, []string, error) {
	prj, err := soundPLANPRJ(opts.CRS)
	if err != nil {
		return SoundPLANReport{}, nil, err
	}

	err = os.MkdirAll(dir, 0o750)
	if err != nil {
		return SoundPLANReport{}, nil, fmt.Errorf("create %s: %w", dir, err)
	}

	report := SoundPLANReport{
		Format:  "esri-shapefile",
		Purpose: "model input for a SoundPLAN calculation; contains no computed levels",
		CRS:     opts.CRS,
		ElevationReference: "HEIGHT_M and HS_M are heights above ground, as the model holds them; " +
			"ground elevation is " + SoundPLANTerrainFile + " when present, never folded into the heights",
		SourceCRS: opts.SourceCRS,
		Layers:    []SoundPLANLayerReport{},
	}

	var (
		paths []string
		lossy []SoundPLANLossyEntry
	)

	byLayer, unplaced := splitSoundPLANLayers(features)
	lossy = append(lossy, unplaced...)

	for _, spec := range soundPLANLayers {
		members := byLayer[spec.name]
		if len(members) == 0 {
			continue
		}

		layer, layerReport, layerLossy := buildSoundPLANLayer(spec, members)
		lossy = append(lossy, layerLossy...)

		if len(layer.Records) == 0 {
			continue
		}

		layer.PRJ = prj
		layer.Date = opts.Date

		written, err := WriteShapefile(filepath.Join(dir, spec.name), layer)
		if err != nil {
			return SoundPLANReport{}, nil, err
		}

		layerReport.File = spec.name + ".shp"
		report.Layers = append(report.Layers, layerReport)
		paths = append(paths, written...)
	}

	slices.SortStableFunc(lossy, func(a, b SoundPLANLossyEntry) int {
		return strings.Compare(a.FeatureID+"\x00"+a.Property+"\x00"+a.Reason, b.FeatureID+"\x00"+b.Property+"\x00"+b.Reason)
	})

	report.Lossy = slices.Compact(lossy)
	if report.Lossy == nil {
		report.Lossy = []SoundPLANLossyEntry{}
	}

	return report, paths, nil
}

// Write stores the report as indented JSON.
func (r SoundPLANReport) Write(path string) error {
	data, err := jsonio.Marshal(r)
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}

	err = atomicfile.WriteFile(path, data)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

func splitSoundPLANLayers(features []ModelFeature) (map[string][]ModelFeature, []SoundPLANLossyEntry) {
	byLayer := make(map[string][]ModelFeature, len(soundPLANLayers))

	var lossy []SoundPLANLossyEntry

	for _, f := range features {
		placed := false

		for _, spec := range soundPLANLayers {
			if f.Kind == spec.kind && (spec.sourceType == "" || f.SourceType == spec.sourceType) {
				byLayer[spec.name] = append(byLayer[spec.name], f)
				placed = true

				break
			}
		}

		if !placed {
			lossy = append(lossy, SoundPLANLossyEntry{
				FeatureID: f.ID,
				Property:  "kind",
				Reason:    fmt.Sprintf("kind %q (source_type %q) has no SoundPLAN layer; the feature is not exported", f.Kind, f.SourceType),
			})
		}
	}

	return byLayer, lossy
}

// soundPLANColumn is a field together with how a property value becomes its
// cell.
type soundPLANColumn struct {
	field    ShapeField
	property string
	unit     string
	// rounded is set for a passthrough number some value of which needs more
	// than passthroughMaxDecimals digits.
	rounded map[string]bool
}

func buildSoundPLANLayer(spec soundPLANLayerSpec, members []ModelFeature) (ShapeLayer, SoundPLANLayerReport, []SoundPLANLossyEntry) {
	columns, lossy := soundPLANColumns(members)

	layer := ShapeLayer{Type: spec.shape}
	report := SoundPLANLayerReport{
		Layer:    spec.name,
		Geometry: shapeTypeName(spec.shape),
		Features: len(members),
	}

	split := spec.shape == ShapePoint && slices.ContainsFunc(members, func(f ModelFeature) bool {
		return f.GeometryType == geoJSONMultiPoint
	})

	fieldColumns := slices.Clone(columns)
	if split {
		fieldColumns = append(fieldColumns, soundPLANColumn{
			field:    ShapeField{Name: partField, Kind: FieldNumeric},
			property: partProperty,
		})
	}

	for _, c := range fieldColumns {
		layer.Fields = append(layer.Fields, c.field)
		report.Columns = append(report.Columns, columnReport(c))
	}

	for _, f := range members {
		parts, hasZ, err := soundPLANGeometry(spec.shape, f)
		if err != nil {
			lossy = append(lossy, SoundPLANLossyEntry{FeatureID: f.ID, Property: geometryProperty, Reason: err.Error() + "; the feature is not exported"})

			continue
		}

		if hasZ {
			lossy = append(lossy, SoundPLANLossyEntry{
				FeatureID: f.ID,
				Property:  geometryProperty,
				Reason:    "vertex z coordinates are dropped; Shapefile layers are written 2D and heights travel as attributes",
			})
		}

		values, valueLossy := soundPLANValues(f, columns)
		lossy = append(lossy, valueLossy...)

		if f.GeometryType == geoJSONMultiPoint {
			// One record per part, each carrying the feature's full values:
			// ISO 9613-2 extraction makes every MultiPoint part a source of its
			// own at the feature's Lw, and names it <id>-NN the same way.
			for i, part := range parts {
				partValues := slices.Clone(values)
				if len(parts) > 1 && values[0] != nil {
					// The suffix can push an ID that fits past the limit.
					partID, reason := soundPLANCell(f.ID, columns[0], fmt.Sprintf("%s-%02d", f.ID, i+1))
					if reason != "" {
						lossy = append(lossy, SoundPLANLossyEntry{FeatureID: f.ID, Property: "id", Reason: reason})
					}

					partValues[0] = partID
				}

				partValues = append(partValues, float64(i+1))
				layer.Records = append(layer.Records, ShapeRecord{Parts: [][][2]float64{part}, Values: partValues})
			}

			continue
		}

		if split {
			values = append(values, nil)
		}

		layer.Records = append(layer.Records, ShapeRecord{Parts: parts, Values: values})
	}

	report.Records = len(layer.Records)

	return layer, report, lossy
}

func columnReport(c soundPLANColumn) SoundPLANColumn {
	out := SoundPLANColumn{Field: c.field.Name, Property: c.property, Unit: c.unit}

	switch c.field.Kind {
	case FieldChar:
		out.Type = "character"
	case FieldNumeric:
		out.Type = "numeric"
		decimals := c.field.Decimals
		out.Decimals = &decimals
	case FieldLogical:
		out.Type = "logical"
	}

	return out
}

// soundPLANColumns settles the layer's column set: ID, the core columns its
// features carry, then every other scalar property in sorted key order.
func soundPLANColumns(members []ModelFeature) ([]soundPLANColumn, []SoundPLANLossyEntry) {
	columns := []soundPLANColumn{{field: ShapeField{Name: idField, Kind: FieldChar}, property: "id"}}
	used := map[string]bool{idField: true, partField: true}
	consumed := maps.Clone(soundPLANImpliedProperties)

	for _, core := range soundPLANCoreColumns {
		consumed[core.property] = true
		used[core.field] = true

		if !anyCarries(members, core.property) {
			continue
		}

		columns = append(columns, soundPLANColumn{
			field:    ShapeField{Name: core.field, Kind: core.kind, Decimals: core.decimals},
			property: core.property,
			unit:     core.unit,
		})
	}

	keys := map[string]bool{}

	for _, f := range members {
		for k := range f.Properties {
			if !consumed[k] {
				keys[k] = true
			}
		}
	}

	var lossy []SoundPLANLossyEntry

	for _, key := range slices.Sorted(maps.Keys(keys)) {
		column, ok, keyLossy := passthroughColumn(members, key, used)
		lossy = append(lossy, keyLossy...)

		if ok {
			columns = append(columns, column)
		}
	}

	return columns, lossy
}

// passthroughColumn is the column a property without a declared mapping gets,
// or false with the reason it gets none.
func passthroughColumn(members []ModelFeature, key string, used map[string]bool) (soundPLANColumn, bool, []SoundPLANLossyEntry) {
	every := func(reason string, holds func(any) bool) []SoundPLANLossyEntry {
		var lossy []SoundPLANLossyEntry

		for _, f := range members {
			if v, has := f.Properties[key]; has && holds(v) {
				lossy = append(lossy, SoundPLANLossyEntry{FeatureID: f.ID, Property: key, Reason: reason})
			}
		}

		return lossy
	}

	kind, agree := passthroughKind(members, key)
	if !agree {
		return soundPLANColumn{}, false, every(
			"the property holds values of different types across features, which no single dBASE field can",
			func(v any) bool { return v != nil },
		)
	}

	if kind == 0 {
		return soundPLANColumn{}, false, nil // only arrays, objects or nulls; reported per feature
	}

	column := soundPLANColumn{field: ShapeField{Kind: kind}, property: key}

	if kind == FieldNumeric {
		column.field.Decimals, column.rounded = passthroughDecimals(members, key)

		if !numericFits(members, key, column.field.Decimals) {
			return soundPLANColumn{}, false, every(
				fmt.Sprintf("a value needs more than %d characters, more than a dBASE numeric field holds", maxDBFNumericWidth),
				func(v any) bool { _, isNumber := v.(float64); return isNumber },
			)
		}
	}

	column.field.Name = allocateFieldName(key, used)

	return column, true, nil
}

func anyCarries(members []ModelFeature, key string) bool {
	for _, f := range members {
		if v, ok := f.Properties[key]; ok && v != nil {
			return true
		}
	}

	return false
}

// passthroughKind is the dBASE type every scalar value of key agrees on. It
// answers 0 when no feature holds a scalar, and false when they disagree.
func passthroughKind(members []ModelFeature, key string) (FieldKind, bool) {
	var kind FieldKind

	for _, f := range members {
		var k FieldKind

		switch f.Properties[key].(type) {
		case string:
			k = FieldChar
		case float64:
			k = FieldNumeric
		case bool:
			k = FieldLogical
		default:
			continue
		}

		if kind != 0 && kind != k {
			return 0, false
		}

		kind = k
	}

	return kind, true
}

// passthroughDecimals is the fewest fractional digits that write every value of
// key exactly, capped at passthroughMaxDecimals. It also returns the features
// whose value the cap rounds.
func passthroughDecimals(members []ModelFeature, key string) (int, map[string]bool) {
	decimals := 0
	rounded := map[string]bool{}

	for _, f := range members {
		v, ok := f.Properties[key].(float64)
		if !ok {
			continue
		}

		text := strconv.FormatFloat(v, 'f', -1, 64)

		need := 0
		if dot := strings.IndexByte(text, '.'); dot >= 0 {
			need = len(text) - dot - 1
		}

		if need > passthroughMaxDecimals {
			rounded[f.ID] = true
			need = passthroughMaxDecimals
		}

		decimals = max(decimals, need)
	}

	return decimals, rounded
}

func numericFits(members []ModelFeature, key string, decimals int) bool {
	for _, f := range members {
		if v, ok := f.Properties[key].(float64); ok && len(strconv.FormatFloat(v, 'f', decimals, 64)) > maxDBFNumericWidth {
			return false
		}
	}

	return true
}

// soundPLANValues returns the cells of one feature, in column order, and what
// of the feature the columns could not hold.
func soundPLANValues(f ModelFeature, columns []soundPLANColumn) ([]any, []SoundPLANLossyEntry) {
	values := make([]any, len(columns))
	inColumn := map[string]bool{}

	var lossy []SoundPLANLossyEntry

	for i, c := range columns {
		inColumn[c.property] = true

		// The ID goes through the same cell rule as any other text: one too
		// long for a character field is left empty and reported, rather than
		// failing the whole export.
		var v any = f.ID

		if c.property != "id" {
			var ok bool

			v, ok = f.Properties[c.property]
			if !ok || v == nil {
				continue
			}
		}

		value, reason := soundPLANCell(f.ID, c, v)
		if reason != "" {
			lossy = append(lossy, SoundPLANLossyEntry{FeatureID: f.ID, Property: c.property, Reason: reason})
		}

		values[i] = value
	}

	for _, key := range slices.Sorted(maps.Keys(f.Properties)) {
		if inColumn[key] || soundPLANImpliedProperties[key] {
			continue
		}

		switch f.Properties[key].(type) {
		case []any, map[string]any:
			lossy = append(lossy, SoundPLANLossyEntry{
				FeatureID: f.ID,
				Property:  key,
				Reason:    "structured values (arrays, objects) have no dBASE field type; the property is not exported",
			})
		}
	}

	return values, lossy
}

// soundPLANCell is the value a column writes for v, and, when the column cannot
// hold it as it is, why. A value it cannot hold at all is written empty.
func soundPLANCell(featureID string, c soundPLANColumn, v any) (any, string) {
	switch c.field.Kind {
	case FieldChar:
		s, ok := v.(string)
		if !ok {
			return nil, "the value is not text, as this column is; the cell is left empty"
		}

		if len(s) > MaxDBFCharBytes {
			return nil, fmt.Sprintf("the value is %d bytes, more than a dBASE character field holds (%d); the cell is left empty", len(s), MaxDBFCharBytes)
		}
	case FieldNumeric:
		if _, ok := v.(float64); !ok {
			return nil, "the value is not a number, as this column is; the cell is left empty"
		}

		if c.rounded[featureID] {
			return v, fmt.Sprintf("the value is rounded to %d decimals", c.field.Decimals)
		}
	case FieldLogical:
		if _, ok := v.(bool); !ok {
			return nil, "the value is not a boolean, as this column is; the cell is left empty"
		}
	}

	return v, ""
}

// soundPLANGeometry converts a feature's GeoJSON coordinates into Shapefile
// parts for the layer's shape type, and says whether any vertex carried a z.
func soundPLANGeometry(shape ShapeType, f ModelFeature) ([][][2]float64, bool, error) {
	var (
		hasZ  bool
		parts [][][2]float64
		err   error
	)

	switch {
	case shape == ShapePoint && (f.GeometryType == geoJSONPoint || f.GeometryType == geoJSONMultiPoint):
		parts, err = pointParts(f, &hasZ)
	case shape == ShapePolyLine && f.GeometryType == geoJSONLineString:
		var line [][2]float64

		line, err = lineOf(f.Coordinates, &hasZ)
		parts = [][][2]float64{line}
	case shape == ShapePolyLine && f.GeometryType == geoJSONMultiLineString:
		parts, err = ringsOf(f.Coordinates, &hasZ)
	case shape == ShapePolygon && (f.GeometryType == geoJSONPolygon || f.GeometryType == geoJSONMultiPolygon):
		parts, err = polygonParts(f, &hasZ)
	default:
		err = fmt.Errorf("geometry %q does not fit a %s layer", f.GeometryType, shapeTypeName(shape))
	}

	if err != nil {
		return nil, false, err
	}

	return parts, hasZ, nil
}

// pointParts is one part per position: a Point is one, a MultiPoint as many as
// it holds.
func pointParts(f ModelFeature, hasZ *bool) ([][][2]float64, error) {
	if f.GeometryType == geoJSONPoint {
		p, err := xyOf(f.Coordinates, hasZ)
		if err != nil {
			return nil, err
		}

		return [][][2]float64{{p}}, nil
	}

	line, err := lineOf(f.Coordinates, hasZ)
	if err != nil {
		return nil, err
	}

	if len(line) == 0 {
		return nil, errors.New("MultiPoint has no position")
	}

	parts := make([][][2]float64, len(line))
	for i, p := range line {
		parts[i] = [][2]float64{p}
	}

	return parts, nil
}

// polygonParts is every ring of a Polygon or MultiPolygon, oriented for the
// Shapefile.
func polygonParts(f ModelFeature, hasZ *bool) ([][][2]float64, error) {
	if f.GeometryType == geoJSONPolygon {
		rings, err := ringsOf(f.Coordinates, hasZ)
		if err != nil {
			return nil, err
		}

		return OrientPolygonRings([][][][2]float64{rings}), nil
	}

	raw, ok := f.Coordinates.([]any)
	if !ok {
		return nil, errors.New("MultiPolygon coordinates are not an array")
	}

	polygons := make([][][][2]float64, 0, len(raw))

	for _, p := range raw {
		rings, err := ringsOf(p, hasZ)
		if err != nil {
			return nil, err
		}

		polygons = append(polygons, rings)
	}

	return OrientPolygonRings(polygons), nil
}

func xyOf(raw any, hasZ *bool) ([2]float64, error) {
	coords, ok := raw.([]any)
	if !ok || len(coords) < 2 {
		return [2]float64{}, errors.New("a position is not an array of at least two numbers")
	}

	x, errX := toFloat64(coords[0])
	y, errY := toFloat64(coords[1])

	if errX != nil || errY != nil {
		return [2]float64{}, errors.New("a position holds a non-numeric coordinate")
	}

	if len(coords) > 2 {
		*hasZ = true
	}

	return [2]float64{x, y}, nil
}

func lineOf(raw any, hasZ *bool) ([][2]float64, error) {
	positions, ok := raw.([]any)
	if !ok {
		return nil, errors.New("a coordinate list is not an array")
	}

	line := make([][2]float64, 0, len(positions))

	for _, p := range positions {
		xy, err := xyOf(p, hasZ)
		if err != nil {
			return nil, err
		}

		line = append(line, xy)
	}

	return line, nil
}

func ringsOf(raw any, hasZ *bool) ([][][2]float64, error) {
	lists, ok := raw.([]any)
	if !ok {
		return nil, errors.New("a ring list is not an array")
	}

	rings := make([][][2]float64, 0, len(lists))

	for _, l := range lists {
		line, err := lineOf(l, hasZ)
		if err != nil {
			return nil, err
		}

		rings = append(rings, line)
	}

	return rings, nil
}

func shapeTypeName(t ShapeType) string {
	switch t {
	case ShapePoint:
		return "Point"
	case ShapePolyLine:
		return "PolyLine"
	case ShapePolygon:
		return "Polygon"
	default:
		return fmt.Sprintf("shape type %d", t)
	}
}

// allocateFieldName derives a dBASE field name from a property key: upper
// case, [A-Z0-9_] only, at most ten characters, and unique within used — a
// collision is resolved with a numeric suffix, which the mapping report then
// resolves back to the key.
func allocateFieldName(key string, used map[string]bool) string {
	var b strings.Builder

	for _, r := range strings.ToUpper(key) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}

	base := b.String()
	if base == "" || base[0] < 'A' || base[0] > 'Z' {
		base = "F_" + base
	}

	candidate := truncateASCII(base, MaxDBFFieldName)

	for n := 1; used[candidate]; n++ {
		suffix := "_" + strconv.Itoa(n)
		candidate = truncateASCII(base, MaxDBFFieldName-len(suffix)) + suffix
	}

	used[candidate] = true

	return candidate
}

func truncateASCII(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n]
}

// WriteSoundPLANTerrain writes elevation points as X;Y;Z lines with a decimal
// comma — the layout of the Höhen.txt files SoundPLAN projects carry, which
// soundplanimport.ParseHoehenTxtFile reads back. Values are written at
// millimetre resolution, which is this file's rounding boundary.
//
// each is called once and hands every point to emit in the order to write
// them. It streams rather than collects, because a DTM at native resolution is
// millions of points.
func WriteSoundPLANTerrain(path string, each func(emit func(x, y, z float64) error) error) (SoundPLANTerrainReport, error) {
	// A temporary name unique per call, as atomicfile.Stage uses: two exports
	// into one bundle must not truncate and rename each other's stream. The
	// stream is why Stage itself does not fit — it takes a finished buffer.
	file, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return SoundPLANTerrainReport{}, fmt.Errorf("create temporary %s: %w", filepath.Base(path), err)
	}

	tmp := file.Name()

	w := bufio.NewWriter(file)
	count := 0

	emitErr := each(func(x, y, z float64) error {
		if !isFinite(x) || !isFinite(y) || !isFinite(z) {
			return fmt.Errorf("elevation point (%v, %v, %v) is not finite", x, y, z)
		}

		count++

		_, err := fmt.Fprintf(w, "%s;%s;%s\n", germanDecimal(x), germanDecimal(y), germanDecimal(z))
		if err != nil {
			return fmt.Errorf("write elevation point: %w", err)
		}

		return nil
	})

	flushErr := w.Flush()
	closeErr := file.Close()

	err = errors.Join(emitErr, flushErr, closeErr)
	if err == nil && count == 0 {
		err = errors.New("the terrain holds no elevation point")
	}

	if err == nil {
		err = os.Rename(tmp, path)
	}

	if err != nil {
		_ = os.Remove(tmp)

		return SoundPLANTerrainReport{}, fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}

	return SoundPLANTerrainReport{
		File:     filepath.Base(path),
		Points:   count,
		Layout:   "X;Y;Z per line, decimal comma, DTM cell centres in row-major order from the north-west corner",
		Decimals: soundPLANTerrainDecimals,
	}, nil
}

func germanDecimal(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', soundPLANTerrainDecimals, 64), ".", ",", 1)
}
