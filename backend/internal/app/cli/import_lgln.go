package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/lglnimport"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/spf13/cobra"
)

// newLGLNClient builds the client `import --from-lgln` downloads with. It is a
// variable so tests can point it at a local STAC server.
var newLGLNClient = func() *lglnimport.Client { return lglnimport.NewClient() }

// lglnImportSummary is what an LGLN import did, for the text and JSON output.
type lglnImportSummary struct {
	Added       int            `json:"added"`
	Replaced    int            `json:"replaced_osm_buildings"`
	Duplicates  int            `json:"skipped_duplicates"`
	Tiles       []string       `json:"tiles"`
	Skipped     map[string]int `json:"skipped"`
	Attribution string         `json:"attribution"`
}

// runLGLNImport loads the LGLN LoD2 buildings for a WGS84 box and merges them
// into the project model instead of replacing it.
//
// The merge is the one the import page performs (planReplaceBuildings in
// frontend/src/model/model-store.ts): the OSM buildings whose footprint
// centroid lies in the box make way, everything else stays — roads, barriers,
// receivers, the calculation area, drawn buildings and the buildings of an
// earlier LGLN load — and an incoming building whose ID the model already
// holds is skipped, so loading the same box twice changes nothing.
func runLGLNImport(
	cmd *cobra.Command,
	state commandState,
	store projectfs.Store,
	proj *project.Project,
	lglnBBox string,
	normalizedPath string,
) (lglnImportSummary, error) {
	bb, err := parseLGLNBBox(lglnBBox)
	if err != nil {
		return lglnImportSummary{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", "invalid --from-lgln bbox: "+err.Error(), err)
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = cmd.Root().Context()
	}

	cacheDir := filepath.Join(store.Root(), ".noise", "cache", "lgln")

	result, err := newLGLNClient().Load(ctx, bb, cacheDir)
	if err != nil {
		return lglnImportSummary{}, lglnLoadError(err)
	}

	payload, err := json.Marshal(result.Collection)
	if err != nil {
		return lglnImportSummary{}, domainerrors.New(domainerrors.KindInternal, "cli.import", "marshal LGLN FeatureCollection", err)
	}

	incoming, err := modelgeojson.NormalizeWithCRS(payload, proj.CRS, "EPSG:4326", "lgln:"+lglnBBox)
	if err != nil {
		return lglnImportSummary{}, domainerrors.New(domainerrors.KindValidation, "cli.import", "invalid geojson from LGLN import", err)
	}

	existing, err := loadExistingModel(store, proj.CRS)
	if err != nil {
		return lglnImportSummary{}, err
	}

	toWGS84, err := wgs84PipelineFrom(proj.CRS)
	if err != nil {
		return lglnImportSummary{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", "project CRS cannot be compared with the LGLN box", err)
	}

	features, stats, err := mergeLGLNBuildings(existing, incoming.Features, bb, toWGS84)
	if err != nil {
		return lglnImportSummary{}, domainerrors.New(domainerrors.KindInternal, "cli.import", "merge LGLN buildings", err)
	}

	merged := incoming
	merged.Features = features

	report, err := validateAndSaveModel(store, proj, merged)
	if err != nil {
		return lglnImportSummary{}, err
	}

	summary := lglnImportSummary{
		Added:       stats.added,
		Replaced:    stats.replaced,
		Duplicates:  stats.duplicates,
		Tiles:       make([]string, 0, len(result.Tiles)),
		Skipped:     result.Skipped,
		Attribution: lglnimport.Attribution(result.Tiles),
	}
	for _, tile := range result.Tiles {
		summary.Tiles = append(summary.Tiles, tile.ID)
	}

	state.Logger.Info(
		"LGLN import completed",
		"bbox", lglnBBox,
		"added", summary.Added,
		"replaced_osm_buildings", summary.Replaced,
		"skipped_duplicates", summary.Duplicates,
		"tiles", len(summary.Tiles),
		outputFieldWarnings, report.WarningCount(),
	)

	printLGLNImportSummary(cmd, store, summary, len(merged.Features), normalizedPath, report)

	return summary, nil
}

// validateAndSaveModel persists a model that validates without errors.
func validateAndSaveModel(store projectfs.Store, proj *project.Project, model modelgeojson.Model) (modelgeojson.ValidationReport, error) {
	report := modelgeojson.Validate(model)
	if report.ErrorCount() > 0 {
		messages := make([]string, 0, len(report.Errors))
		for _, issue := range report.Errors {
			messages = append(messages, issue.Code+": "+issue.Message)
		}

		return report, domainerrors.New(domainerrors.KindValidation, "cli.import", summarizeValidationErrors(messages, 3), nil)
	}

	err := store.SaveModel(proj, model, report)
	if err != nil {
		return report, fmt.Errorf("persist model artifacts: %w", err)
	}

	return report, nil
}

func printLGLNImportSummary(
	cmd *cobra.Command,
	store projectfs.Store,
	summary lglnImportSummary,
	featureCount int,
	normalizedPath string,
	report modelgeojson.ValidationReport,
) {
	out := cmd.OutOrStdout()

	_, _ = fmt.Fprintf(out, "Imported %d LGLN LoD2 buildings from %d tiles\n", summary.Added, len(summary.Tiles))

	if summary.Replaced > 0 {
		_, _ = fmt.Fprintf(out, "Replaced OSM buildings: %d\n", summary.Replaced)
	}

	if summary.Duplicates > 0 {
		_, _ = fmt.Fprintf(out, "Already in the model: %d\n", summary.Duplicates)
	}

	reasons := make([]string, 0, len(summary.Skipped))
	for reason := range summary.Skipped {
		reasons = append(reasons, reason)
	}

	sort.Strings(reasons)

	for _, reason := range reasons {
		_, _ = fmt.Fprintf(out, "Skipped (%s): %d\n", reason, summary.Skipped[reason])
	}

	_, _ = fmt.Fprintf(out, "Model features: %d\n", featureCount)
	_, _ = fmt.Fprintf(out, "Normalized GeoJSON: %s\n", relativePath(store.Root(), normalizedPath))

	if report.WarningCount() > 0 {
		_, _ = fmt.Fprintf(out, "Validation warnings: %d\n", report.WarningCount())
	}

	if summary.Attribution != "" {
		_, _ = fmt.Fprintln(out, summary.Attribution)
	}
}

// parseLGLNBBox reads the "south,west,north,east" spelling --from-osm uses.
func parseLGLNBBox(s string) (lglnimport.BBox, error) {
	osmBox, err := parseOSMBBox(s)
	if err != nil {
		return lglnimport.BBox{}, err
	}

	return lglnimport.BBox{West: osmBox.West, South: osmBox.South, East: osmBox.East, North: osmBox.North}, nil
}

// lglnLoadError classifies a failed Load the way the HTTP handler does: a bad
// box is the user's to fix, everything else lies with the download service.
// Each message is followed by the cause, which carries the specifics.
func lglnLoadError(err error) error {
	var tooMany *lglnimport.TooManyTilesError

	switch {
	case errors.As(err, &tooMany):
		return domainerrors.New(domainerrors.KindUserInput, "cli.import",
			"choose a smaller --from-lgln box: one import covers at most nine 1 km tiles", err)
	case errors.Is(err, lglnimport.ErrOutsideCoverage):
		return domainerrors.New(domainerrors.KindUserInput, "cli.import",
			"LGLN publishes LoD2 buildings for Lower Saxony only", err)
	case errors.Is(err, lglnimport.ErrInvalidBBox):
		return domainerrors.New(domainerrors.KindUserInput, "cli.import", "invalid --from-lgln bbox", err)
	default:
		return domainerrors.New(domainerrors.KindUserInput, "cli.import", "LGLN LoD2 download failed", err)
	}
}

// loadExistingModel returns the features of the saved model, or none when the
// project has no model yet.
func loadExistingModel(store projectfs.Store, projectCRS string) ([]modelgeojson.Feature, error) {
	raw, err := store.ReadModel()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("read existing model: %w", err)
	}

	model, err := modelgeojson.Normalize(raw, projectCRS, "")
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindInternal, "cli.import", "decode normalized model", err)
	}

	return model.Features, nil
}

// wgs84PipelineFrom transforms project coordinates into WGS84 degrees, the CRS
// the LGLN box is given in.
func wgs84PipelineFrom(projectCRS string) (geo.TransformPipeline, error) {
	from, err := geo.ParseCRS(projectCRS)
	if err != nil {
		return geo.TransformPipeline{}, fmt.Errorf("parse project CRS %q: %w", projectCRS, err)
	}

	to, err := geo.ParseCRS("EPSG:4326")
	if err != nil {
		return geo.TransformPipeline{}, fmt.Errorf("parse EPSG:4326: %w", err)
	}

	pipeline, err := geo.BuildTransformPipeline(to, from)
	if err != nil {
		return geo.TransformPipeline{}, fmt.Errorf("build %s -> EPSG:4326 transform: %w", projectCRS, err)
	}

	return pipeline, nil
}

type lglnMergeStats struct {
	added, replaced, duplicates int
}

// mergeLGLNBuildings drops the OSM buildings whose footprint centroid lies in
// bb, then appends every incoming feature whose ID is not taken yet. Existing
// features keep their order; incoming ones follow in the order Load returned.
//
// The centroid is taken in the project CRS and then transformed to WGS84 —
// the order lglnimport.Load uses for the LGLN buildings, which it computes in
// the tile's UTM CRS.
func mergeLGLNBuildings(
	existing, incoming []modelgeojson.Feature,
	bb lglnimport.BBox,
	toWGS84 geo.TransformPipeline,
) ([]modelgeojson.Feature, lglnMergeStats, error) {
	var stats lglnMergeStats

	out := make([]modelgeojson.Feature, 0, len(existing)+len(incoming))
	taken := make(map[string]struct{}, len(existing)+len(incoming))

	for _, feature := range existing {
		replaced, err := replacedByLGLN(feature, bb, toWGS84)
		if err != nil {
			return nil, lglnMergeStats{}, err
		}

		if replaced {
			stats.replaced++

			continue
		}

		taken[feature.ID] = struct{}{}
		out = append(out, feature)
	}

	for _, feature := range incoming {
		if _, dup := taken[feature.ID]; dup {
			stats.duplicates++

			continue
		}

		taken[feature.ID] = struct{}{}
		out = append(out, feature)
		stats.added++
	}

	return out, stats, nil
}

// replacedByLGLN reports whether an existing feature is an OSM building whose
// footprint centroid lies in the LGLN box.
func replacedByLGLN(feature modelgeojson.Feature, bb lglnimport.BBox, toWGS84 geo.TransformPipeline) (bool, error) {
	if !isOSMBuilding(feature) {
		return false, nil
	}

	centroid, ok := footprintCentroid(feature)
	if !ok {
		return false, nil
	}

	lonLat, err := toWGS84.ApplyPoint(centroid)
	if err != nil {
		return false, fmt.Errorf("feature %s: %w", feature.ID, err)
	}

	return bb.Contains(lonLat), nil
}

// isOSMBuilding mirrors the frontend's isOSMBuilding: the OSM importer mints
// `osm-way-<id>` and sets `osm_id`, and either mark counts. A building that
// says it came from LGLN never is one.
func isOSMBuilding(feature modelgeojson.Feature) bool {
	if feature.Kind != modelgeojson.FeatureKindBuilding {
		return false
	}

	if format, _ := feature.Properties["import_format"].(string); format == lglnimport.ImportFormat {
		return false
	}

	_, hasOSMID := feature.Properties["osm_id"]

	return strings.HasPrefix(feature.ID, "osm-way-") || hasOSMID
}

// footprintCentroid is the area centroid of a Polygon, or the area-weighted
// centroid of a MultiPolygon's parts, in the model's own coordinates.
func footprintCentroid(feature modelgeojson.Feature) (geo.Point2D, bool) {
	var polygons []any

	switch feature.GeometryType {
	case "Polygon":
		polygons = []any{feature.Coordinates}
	case "MultiPolygon":
		parts, ok := feature.Coordinates.([]any)
		if !ok {
			return geo.Point2D{}, false
		}

		polygons = parts
	default:
		return geo.Point2D{}, false
	}

	var area, x, y float64

	for _, polygon := range polygons {
		rings, ok := anyPolygonRings(polygon)
		if !ok {
			continue
		}

		centroid, ok := geo.PolygonCentroid(rings)
		if !ok {
			continue
		}

		a := geo.PolygonArea(rings)
		area += a
		x += centroid.X * a
		y += centroid.Y * a
	}

	if area <= 0 {
		return geo.Point2D{}, false
	}

	return geo.Point2D{X: x / area, Y: y / area}, true
}

// anyPolygonRings reads decoded-JSON polygon coordinates.
func anyPolygonRings(coords any) ([][]geo.Point2D, bool) {
	rawRings, ok := coords.([]any)
	if !ok || len(rawRings) == 0 {
		return nil, false
	}

	rings := make([][]geo.Point2D, 0, len(rawRings))

	for _, rawRing := range rawRings {
		rawPoints, ok := rawRing.([]any)
		if !ok {
			return nil, false
		}

		ring := make([]geo.Point2D, 0, len(rawPoints))

		for _, rawPoint := range rawPoints {
			pair, ok := rawPoint.([]any)
			if !ok || len(pair) < 2 {
				return nil, false
			}

			px, okX := pair[0].(float64)
			py, okY := pair[1].(float64)

			if !okX || !okY {
				return nil, false
			}

			ring = append(ring, geo.Point2D{X: px, Y: py})
		}

		rings = append(rings, ring)
	}

	return rings, true
}
