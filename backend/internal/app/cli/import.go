package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/io/citygmlimport"
	"github.com/aconiq/backend/internal/io/csvimport"
	"github.com/aconiq/backend/internal/io/fgbimport"
	"github.com/aconiq/backend/internal/io/gpkgimport"
	"github.com/aconiq/backend/internal/io/osmimport"
	"github.com/aconiq/backend/internal/io/projectfs"
	"github.com/spf13/cobra"
)

func newImportCommand() *cobra.Command {
	var inputPath string
	var layerName string
	var inputCRS string
	var trafficPath string
	var terrainPath string
	var osmBBox string
	var osmEndpoint string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import GeoJSON, GeoPackage, FlatGeobuf, or CityGML model data into the project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runImport(cmd, inputPath, layerName, inputCRS, trafficPath, terrainPath, osmBBox, osmEndpoint)
		},
	}

	cmd.Flags().StringVar(&inputPath, "input", "", "Path to GeoJSON, GeoPackage (.gpkg), FlatGeobuf (.fgb), or CityGML (.gml/.citygml) input file")
	cmd.Flags().StringVar(&layerName, "layer", "", "Layer name to import from a GeoPackage file (required for .gpkg)")
	cmd.Flags().StringVar(&inputCRS, "input-crs", "", "CRS of the input data (e.g. EPSG:25832); auto-detected from GeoPackage/FlatGeobuf/CityGML if omitted")
	cmd.Flags().StringVar(&trafficPath, "traffic", "", "Path to CSV attribute table for merging into the model")
	cmd.Flags().StringVar(&terrainPath, "terrain", "", "Path to GeoTIFF DTM file for terrain elevation data")
	cmd.Flags().StringVar(&osmBBox, "from-osm", "", "Bounding box for OSM import: \"south,west,north,east\" in WGS84 degrees")
	cmd.Flags().StringVar(&osmEndpoint, "overpass-endpoint", "", "Overpass API endpoint (optional, defaults to overpass-api.de)")

	return cmd
}

func runImport(cmd *cobra.Command, inputPath, layerName, inputCRS, trafficPath, terrainPath, osmBBox, osmEndpoint string) error {
	err := validateImportFlags(inputPath, trafficPath, terrainPath, osmBBox)
	if err != nil {
		return err
	}

	state, ok := stateFromCommand(cmd)
	if !ok {
		return domainerrors.New(domainerrors.KindInternal, "cli.import", "command state unavailable", nil)
	}

	store, err := projectfs.New(state.Config.ProjectPath)
	if err != nil {
		return err
	}

	proj, err := store.Load()
	if err != nil {
		return err
	}

	modelDir := filepath.Join(store.Root(), ".noise", "model")
	normalizedPath := filepath.Join(modelDir, "model.normalized.geojson")
	dumpPath := filepath.Join(modelDir, "model.dump.json")
	reportPath := filepath.Join(modelDir, "validation-report.json")

	// When JSON output is enabled, suppress human-readable output from
	// sub-functions and emit a single JSON object at the end.
	jsonMode := state.Config.JSONLogs

	var origOut io.Writer
	if jsonMode {
		origOut = cmd.OutOrStdout()
		cmd.SetOut(io.Discard)
	}

	switch {
	case osmBBox != "":
		err = runOSMImport(cmd, state, store, &proj, osmBBox, osmEndpoint, normalizedPath, dumpPath, reportPath)
	case inputPath != "":
		err = runGeometryImport(cmd, state, store, &proj, inputPath, layerName, inputCRS, normalizedPath, dumpPath, reportPath)
	}

	if err == nil && trafficPath != "" {
		err = mergeTrafficCSV(cmd, state, trafficPath, normalizedPath, dumpPath, store.Root())
	}

	if err == nil && terrainPath != "" {
		err = runTerrainImport(cmd, state, store, &proj, terrainPath)
	}

	if jsonMode && err == nil {
		cmd.SetOut(origOut)

		return writeCommandOutput(
			cmd.OutOrStdout(), true,
			buildImportJSONResult(store.Root(), inputPath, osmBBox, trafficPath, terrainPath, normalizedPath, dumpPath, reportPath),
		)
	}

	return err
}

// buildImportJSONResult constructs the JSON payload for `noise --json import`.
// It reads back artifact files already written to disk to populate counts.
func buildImportJSONResult(root, inputPath, osmBBox, trafficPath, terrainPath, normalizedPath, dumpPath, reportPath string) map[string]any {
	result := map[string]any{"command": "import"}

	if inputPath != "" {
		result["input"] = relativePath(root, resolvePath(root, inputPath))
	}

	if osmBBox != "" {
		result["osm_bbox"] = osmBBox
	}

	result["normalized_path"] = relativePath(root, normalizedPath)
	result["dump_path"] = relativePath(root, dumpPath)
	result["report_path"] = relativePath(root, reportPath)

	// Read feature count from the normalized model on disk.
	data, readErr := os.ReadFile(normalizedPath)
	if readErr == nil {
		var fc struct {
			Features []json.RawMessage `json:"features"`
		}

		if json.Unmarshal(data, &fc) == nil {
			result["feature_count"] = len(fc.Features)
		}
	}

	if trafficPath != "" {
		result["traffic_input"] = relativePath(root, resolvePath(root, trafficPath))
	}

	if terrainPath != "" {
		result["terrain_input"] = relativePath(root, resolvePath(root, terrainPath))
		result["terrain_stored_path"] = defaultTerrainPath
	}

	return result
}

func validateImportFlags(inputPath, trafficPath, terrainPath, osmBBox string) error {
	if osmBBox != "" && inputPath != "" {
		return domainerrors.New(domainerrors.KindUserInput, "cli.import", "cannot use --from-osm together with --input", nil)
	}

	if inputPath == "" && trafficPath == "" && osmBBox == "" && terrainPath == "" {
		return domainerrors.New(domainerrors.KindUserInput, "cli.import", "--input, --from-osm, --terrain, or --traffic is required", nil)
	}

	return nil
}

// parseOSMBBox parses a "south,west,north,east" string into a BBox.
func parseOSMBBox(s string) (osmimport.BBox, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return osmimport.BBox{}, fmt.Errorf("expected 4 comma-separated values (south,west,north,east), got %d", len(parts))
	}

	vals := make([]float64, 4)

	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return osmimport.BBox{}, fmt.Errorf("invalid coordinate at position %d: %w", i+1, err)
		}

		vals[i] = v
	}

	return osmimport.BBox{South: vals[0], West: vals[1], North: vals[2], East: vals[3]}, nil
}

func runOSMImport(
	cmd *cobra.Command,
	state commandState,
	store projectfs.Store,
	proj *project.Project,
	osmBBox string,
	osmEndpoint string,
	normalizedPath string,
	dumpPath string,
	reportPath string,
) error {
	model, report, err := fetchAndNormalizeOSM(cmd, proj, osmBBox, osmEndpoint)
	if err != nil {
		return err
	}

	return writeOSMImportArtifacts(cmd, state, store, proj, model, report, osmBBox, normalizedPath, dumpPath, reportPath)
}

// fetchAndNormalizeOSM fetches OSM data and normalizes it into the model.
func fetchAndNormalizeOSM(
	cmd *cobra.Command,
	proj *project.Project,
	osmBBox string,
	osmEndpoint string,
) (modelgeojson.Model, modelgeojson.ValidationReport, error) {
	bbox, err := parseOSMBBox(osmBBox)
	if err != nil {
		return modelgeojson.Model{}, modelgeojson.ValidationReport{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", "invalid --from-osm bbox: "+err.Error(), err)
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = cmd.Root().Context()
	}

	fc, err := osmimport.Fetch(ctx, osmimport.Config{
		BBox:             bbox,
		OverpassEndpoint: osmEndpoint,
	})
	if err != nil {
		return modelgeojson.Model{}, modelgeojson.ValidationReport{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", "OSM/Overpass query failed", err)
	}

	payload, err := json.Marshal(fc)
	if err != nil {
		return modelgeojson.Model{}, modelgeojson.ValidationReport{}, domainerrors.New(domainerrors.KindInternal, "cli.import", "marshal OSM FeatureCollection", err)
	}

	model, err := modelgeojson.NormalizeWithCRS(payload, proj.CRS, "EPSG:4326", "osm:"+osmBBox)
	if err != nil {
		return modelgeojson.Model{}, modelgeojson.ValidationReport{}, domainerrors.New(domainerrors.KindValidation, "cli.import", "invalid geojson from OSM import", err)
	}

	report := modelgeojson.Validate(model)
	if report.ErrorCount() > 0 {
		messages := make([]string, 0, len(report.Errors))
		for _, issue := range report.Errors {
			messages = append(messages, issue.Code+": "+issue.Message)
		}

		return modelgeojson.Model{}, modelgeojson.ValidationReport{}, domainerrors.New(domainerrors.KindValidation, "cli.import", summarizeValidationErrors(messages, 3), nil)
	}

	return model, report, nil
}

// writeOSMImportArtifacts writes model artifacts to disk, updates the project manifest,
// and prints a summary.
func writeOSMImportArtifacts(
	cmd *cobra.Command,
	state commandState,
	store projectfs.Store,
	proj *project.Project,
	model modelgeojson.Model,
	report modelgeojson.ValidationReport,
	osmBBox string,
	normalizedPath string,
	dumpPath string,
	reportPath string,
) error {
	err := writeJSONFile(normalizedPath, model.ToFeatureCollection())
	if err != nil {
		return err
	}

	err = writeJSONFile(dumpPath, model.ToDump())
	if err != nil {
		return err
	}

	err = writeJSONFile(reportPath, report)
	if err != nil {
		return err
	}

	now := nowUTC()
	proj.Artifacts = upsertArtifact(proj.Artifacts, project.ArtifactRef{
		ID:        "artifact-model-normalized",
		Kind:      "model.normalized_geojson",
		Path:      relativePath(store.Root(), normalizedPath),
		CreatedAt: now,
	})
	proj.Artifacts = upsertArtifact(proj.Artifacts, project.ArtifactRef{
		ID:        "artifact-model-dump",
		Kind:      "model.dump_json",
		Path:      relativePath(store.Root(), dumpPath),
		CreatedAt: now,
	})
	proj.Artifacts = upsertArtifact(proj.Artifacts, project.ArtifactRef{
		ID:        "artifact-model-validation",
		Kind:      "model.validation_report",
		Path:      relativePath(store.Root(), reportPath),
		CreatedAt: now,
	})

	err = store.Save(*proj)
	if err != nil {
		return err
	}

	state.Logger.Info(
		"OSM import completed",
		"bbox", osmBBox,
		"feature_count", len(model.Features),
		"warnings", report.WarningCount(),
		"normalized", relativePath(store.Root(), normalizedPath),
	)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Imported %d features from OSM bbox %s\n", len(model.Features), osmBBox)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Normalized GeoJSON: %s\n", relativePath(store.Root(), normalizedPath))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Model dump: %s\n", relativePath(store.Root(), dumpPath))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Validation report: %s\n", relativePath(store.Root(), reportPath))

	if report.WarningCount() > 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Validation warnings: %d\n", report.WarningCount())
	}

	return nil
}

func runGeometryImport(
	cmd *cobra.Command,
	state commandState,
	store projectfs.Store,
	proj *project.Project,
	inputPath string,
	layerName string,
	inputCRS string,
	normalizedPath string,
	dumpPath string,
	reportPath string,
) error {
	absoluteInput := resolvePath(store.Root(), inputPath)
	relInput := relativePath(store.Root(), absoluteInput)

	result, err := loadInputPayload(absoluteInput, layerName)
	if err != nil {
		return err
	}

	// Determine effective import CRS: explicit flag > auto-detected > empty.
	effectiveCRS := inputCRS
	if effectiveCRS == "" && result.detectedCRS != "" {
		effectiveCRS = result.detectedCRS
	}

	model, err := modelgeojson.NormalizeWithCRS(result.payload, proj.CRS, effectiveCRS, relInput)
	if err != nil {
		return domainerrors.New(domainerrors.KindValidation, "cli.import", "invalid geojson input", err)
	}

	report := modelgeojson.Validate(model)
	if report.ErrorCount() > 0 {
		messages := make([]string, 0, len(report.Errors))
		for _, issue := range report.Errors {
			messages = append(messages, issue.Code+": "+issue.Message)
		}

		return domainerrors.New(domainerrors.KindValidation, "cli.import", summarizeValidationErrors(messages, 3), nil)
	}

	err = persistModelArtifacts(store, proj, model, report, normalizedPath, dumpPath, reportPath)
	if err != nil {
		return err
	}

	if result.citygmlReport != nil {
		citygmlReportPath := filepath.Join(filepath.Dir(normalizedPath), "citygml-import-report.json")

		err = writeJSONFile(citygmlReportPath, result.citygmlReport)
		if err != nil {
			return err
		}

		printCityGMLImportReport(cmd, *result.citygmlReport)
	}

	printImportSummary(cmd, state, model, report, relInput, effectiveCRS, store.Root(), normalizedPath, dumpPath, reportPath)

	return nil
}

func persistModelArtifacts(
	store projectfs.Store, proj *project.Project,
	model modelgeojson.Model, report modelgeojson.ValidationReport,
	normalizedPath, dumpPath, reportPath string,
) error {
	err := writeJSONFile(normalizedPath, model.ToFeatureCollection())
	if err != nil {
		return err
	}

	err = writeJSONFile(dumpPath, model.ToDump())
	if err != nil {
		return err
	}

	err = writeJSONFile(reportPath, report)
	if err != nil {
		return err
	}

	now := nowUTC()
	for _, ref := range []project.ArtifactRef{
		{ID: "artifact-model-normalized", Kind: "model.normalized_geojson", Path: relativePath(store.Root(), normalizedPath), CreatedAt: now},
		{ID: "artifact-model-dump", Kind: "model.dump_json", Path: relativePath(store.Root(), dumpPath), CreatedAt: now},
		{ID: "artifact-model-validation", Kind: "model.validation_report", Path: relativePath(store.Root(), reportPath), CreatedAt: now},
	} {
		proj.Artifacts = upsertArtifact(proj.Artifacts, ref)
	}

	return store.Save(*proj)
}

func printImportSummary(
	cmd *cobra.Command, state commandState,
	model modelgeojson.Model, report modelgeojson.ValidationReport,
	relInput, effectiveCRS, root, normalizedPath, dumpPath, reportPath string,
) {
	logFields := []any{
		"input", relInput,
		"feature_count", len(model.Features),
		"warnings", report.WarningCount(),
		"normalized", relativePath(root, normalizedPath),
	}

	if model.TransformApplied {
		logFields = append(logFields, "import_crs", model.ImportCRS, "project_crs", model.ProjectCRS)
	}

	state.Logger.Info("import completed", logFields...)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Imported %d features from %s\n", len(model.Features), relInput)

	if model.TransformApplied {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "CRS transform: %s -> %s\n", model.ImportCRS, model.ProjectCRS)
	} else if effectiveCRS != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Input CRS: %s (matches project CRS, no transform needed)\n", effectiveCRS)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Normalized GeoJSON: %s\n", relativePath(root, normalizedPath))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Model dump: %s\n", relativePath(root, dumpPath))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Validation report: %s\n", relativePath(root, reportPath))

	if report.WarningCount() > 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Validation warnings: %d\n", report.WarningCount())
	}
}

func printCityGMLImportReport(cmd *cobra.Command, r citygmlimport.ImportReport) {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "CityGML: imported %d/%d buildings", r.Imported, r.Total)
	if r.Skipped > 0 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), ", %d skipped", r.Skipped)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout())

	for _, s := range r.Details {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  skipped %s: %s\n", s.ID, s.Reason)
	}
}

// inputPayload holds the raw GeoJSON bytes and optional auto-detected CRS.
type inputPayload struct {
	payload       []byte
	detectedCRS   string                      // e.g. "EPSG:25832", empty if not detected
	citygmlReport *citygmlimport.ImportReport // non-nil for CityGML imports
}

func epsgToString(code int) string {
	if code <= 0 {
		return ""
	}

	return fmt.Sprintf("EPSG:%d", code)
}

// loadInputPayload reads the input file as GeoJSON bytes with optional CRS auto-detection.
// For .gpkg files it uses gpkgimport; for .fgb files it uses fgbimport;
// for .gml/.citygml files it uses citygmlimport; for all others it reads the file directly as GeoJSON.
func loadInputPayload(absoluteInput string, layerName string) (inputPayload, error) {
	ext := strings.ToLower(filepath.Ext(absoluteInput))

	switch ext {
	case ".gpkg":
		return readGPKGAsGeoJSON(absoluteInput, layerName)
	case ".fgb":
		return readFGBAsGeoJSON(absoluteInput, layerName)
	case ".gml", ".citygml", ".xml":
		return readCityGMLAsGeoJSON(absoluteInput, layerName)
	default:
		if layerName != "" {
			return inputPayload{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", "--layer is only valid for GeoPackage (.gpkg) files", nil)
		}

		payload, err := os.ReadFile(absoluteInput)
		if err != nil {
			return inputPayload{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", "read input file: "+absoluteInput, err)
		}

		return inputPayload{payload: payload}, nil
	}
}

// readGPKGAsGeoJSON opens a GeoPackage file and returns its layer as marshalled GeoJSON bytes.
func readGPKGAsGeoJSON(path string, layerName string) (inputPayload, error) {
	if layerName == "" {
		layers, err := gpkgimport.ListLayers(path)
		if err != nil {
			return inputPayload{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", "list GeoPackage layers", err)
		}

		names := make([]string, 0, len(layers))
		for _, l := range layers {
			names = append(names, l.Name)
		}

		msg := "--layer is required for GeoPackage files; available layers: " + strings.Join(names, ", ")

		return inputPayload{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", msg, nil)
	}

	result, err := gpkgimport.ReadLayerWithCRS(path, layerName)
	if err != nil {
		return inputPayload{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", "read GeoPackage layer "+layerName, err)
	}

	payload, err := json.Marshal(result.Collection)
	if err != nil {
		return inputPayload{}, domainerrors.New(domainerrors.KindInternal, "cli.import", "marshal GeoPackage layer to GeoJSON", err)
	}

	return inputPayload{payload: payload, detectedCRS: epsgToString(result.EPSGCode)}, nil
}

// readFGBAsGeoJSON opens a FlatGeobuf file and returns its features as marshalled GeoJSON bytes.
func readFGBAsGeoJSON(path string, layerName string) (inputPayload, error) {
	if layerName != "" {
		return inputPayload{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", "--layer is not supported for FlatGeobuf (.fgb) files", nil)
	}

	result, err := fgbimport.ReadWithCRS(path)
	if err != nil {
		return inputPayload{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", "read FlatGeobuf file", err)
	}

	payload, err := json.Marshal(result.Collection)
	if err != nil {
		return inputPayload{}, domainerrors.New(domainerrors.KindInternal, "cli.import", "marshal FlatGeobuf to GeoJSON", err)
	}

	return inputPayload{payload: payload, detectedCRS: epsgToString(result.EPSGCode)}, nil
}

// readCityGMLAsGeoJSON opens a CityGML file and returns supported features as marshalled GeoJSON bytes.
func readCityGMLAsGeoJSON(path string, layerName string) (inputPayload, error) {
	if layerName != "" {
		return inputPayload{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", "--layer is not supported for CityGML files", nil)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return inputPayload{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", "read CityGML file: "+path, err)
	}

	result, err := citygmlimport.ReadWithCRS(raw)
	if err != nil {
		return inputPayload{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", "read CityGML file", err)
	}

	encoded, err := json.Marshal(result.Collection)
	if err != nil {
		return inputPayload{}, domainerrors.New(domainerrors.KindInternal, "cli.import", "marshal CityGML to GeoJSON", err)
	}

	report := result.Report

	return inputPayload{payload: encoded, detectedCRS: epsgToString(result.EPSGCode), citygmlReport: &report}, nil
}

const defaultTerrainPath = ".noise/model/terrain.tif"

// runTerrainImport validates a GeoTIFF DTM, copies it into the project, and registers it as an artifact.
func runTerrainImport(
	cmd *cobra.Command,
	state commandState,
	store projectfs.Store,
	proj *project.Project,
	terrainPath string,
) error {
	absoluteTerrain := resolvePath(store.Root(), terrainPath)

	ext := strings.ToLower(filepath.Ext(absoluteTerrain))
	if ext != ".tif" && ext != ".tiff" {
		return domainerrors.New(domainerrors.KindUserInput, "cli.import", "--terrain requires a GeoTIFF (.tif/.tiff) file", nil)
	}

	// Validate: load to check it's a readable elevation raster.
	tm, err := terrain.Load(absoluteTerrain)
	if err != nil {
		return domainerrors.New(domainerrors.KindUserInput, "cli.import", "invalid terrain GeoTIFF", err)
	}

	bounds := tm.Bounds()

	// Copy to project model directory.
	destPath := filepath.Join(store.Root(), defaultTerrainPath)

	srcData, err := os.ReadFile(absoluteTerrain)
	if err != nil {
		return domainerrors.New(domainerrors.KindUserInput, "cli.import", "read terrain file", err)
	}

	err = os.MkdirAll(filepath.Dir(destPath), 0o755)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.import", "create model directory", err)
	}

	err = os.WriteFile(destPath, srcData, 0o600)
	if err != nil {
		return domainerrors.New(domainerrors.KindInternal, "cli.import", "write terrain file", err)
	}

	// Register artifact.
	now := nowUTC()
	proj.Artifacts = upsertArtifact(proj.Artifacts, project.ArtifactRef{
		ID:        "artifact-terrain",
		Kind:      "model.terrain_geotiff",
		Path:      defaultTerrainPath,
		CreatedAt: now,
	})

	err = store.Save(*proj)
	if err != nil {
		return err
	}

	state.Logger.Info(
		"terrain import completed",
		"input", relativePath(store.Root(), absoluteTerrain),
		"bounds", fmt.Sprintf("[%.2f, %.2f, %.2f, %.2f]", bounds[0], bounds[1], bounds[2], bounds[3]),
		"dest", defaultTerrainPath,
	)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Imported terrain DTM from %s\n", relativePath(store.Root(), absoluteTerrain))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Bounds: [%.2f, %.2f, %.2f, %.2f]\n", bounds[0], bounds[1], bounds[2], bounds[3])
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Stored: %s\n", defaultTerrainPath)

	return nil
}

// mergeTrafficCSV reads a CSV file and merges its properties into the normalized model.
func mergeTrafficCSV(cmd *cobra.Command, state commandState, trafficPath string, normalizedPath string, dumpPath string, root string) error {
	absTraffic := resolvePath(root, trafficPath)

	records, err := readCSVRecords(absTraffic)
	if err != nil {
		return err
	}

	fc, err := loadNormalizedModel(normalizedPath)
	if err != nil {
		return err
	}

	matched, unmatched := applyCSVRecords(fc, records)

	err = writeJSONFile(normalizedPath, fc)
	if err != nil {
		return err
	}

	refreshDump(state, fc, dumpPath)

	state.Logger.Info(
		"traffic CSV merged",
		"csv", relativePath(root, absTraffic),
		"matched", matched,
		"unmatched", unmatched,
	)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Traffic CSV merged: %d matched, %d unmatched features\n", matched, unmatched)

	return nil
}

func readCSVRecords(absPath string) ([]csvimport.Record, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindUserInput, "cli.import", "open traffic CSV: "+absPath, err)
	}

	defer f.Close()

	records, err := csvimport.ReadTable(f)
	if err != nil {
		return nil, domainerrors.New(domainerrors.KindUserInput, "cli.import", "read traffic CSV", err)
	}

	return records, nil
}

func loadNormalizedModel(normalizedPath string) (modelgeojson.FeatureCollection, error) {
	payload, err := os.ReadFile(normalizedPath)
	if err != nil {
		return modelgeojson.FeatureCollection{}, domainerrors.New(domainerrors.KindUserInput, "cli.import", "read normalized model (run import --input first)", err)
	}

	var fc modelgeojson.FeatureCollection

	err = json.Unmarshal(payload, &fc)
	if err != nil {
		return modelgeojson.FeatureCollection{}, domainerrors.New(domainerrors.KindInternal, "cli.import", "decode normalized model", err)
	}

	return fc, nil
}

func applyCSVRecords(fc modelgeojson.FeatureCollection, records []csvimport.Record) (matched, unmatched int) {
	idToIdx := make(map[string]int, len(fc.Features))
	for i, feat := range fc.Features {
		fid := featID(feat)
		if fid != "" {
			idToIdx[fid] = i
		}
	}

	for _, rec := range records {
		idx, ok := idToIdx[rec.FeatureID]
		if !ok {
			unmatched++

			continue
		}

		if fc.Features[idx].Properties == nil {
			fc.Features[idx].Properties = make(map[string]any)
		}

		maps.Copy(fc.Features[idx].Properties, rec.Properties)

		matched++
	}

	return matched, unmatched
}

func refreshDump(state commandState, fc modelgeojson.FeatureCollection, dumpPath string) {
	updatedPayload, err := json.Marshal(fc)
	if err != nil {
		state.Logger.Warn("could not re-marshal model for dump update", "error", err)

		return
	}

	model, err := modelgeojson.Normalize(updatedPayload, "", "")
	if err != nil {
		state.Logger.Warn("could not re-normalize model for dump update", "error", err)

		return
	}

	writeErr := writeJSONFile(dumpPath, model.ToDump())
	if writeErr != nil {
		state.Logger.Warn("could not update model dump after traffic merge", "error", writeErr)
	}
}

// featID extracts the feature ID from a GeoJSON feature.
func featID(feat modelgeojson.GeoJSONFeature) string {
	if feat.Properties != nil {
		if id, ok := feat.Properties["id"]; ok {
			if s, ok2 := id.(string); ok2 && s != "" {
				return s
			}
		}
	}

	if feat.ID != nil {
		switch v := feat.ID.(type) {
		case string:
			return v
		case float64:
			return fmt.Sprintf("%g", v)
		}
	}

	return ""
}
