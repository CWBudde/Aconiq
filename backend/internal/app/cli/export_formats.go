// GIS format exports for `aconiq export --format`.
//
// Split out of export.go, which carries the command, the bundle staging and
// the report generation. These are the georeferenced outputs — GeoTIFF, COG,
// GeoPackage and contours — plus the one decision they all depend on: where
// the raster sits on the ground.

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	exportfmt "github.com/aconiq/backend/internal/report/export"
	"github.com/aconiq/backend/internal/report/results"
)

// formatExportContext holds the data shared by all per-format export helpers.
//
// A bundle can legitimately hold two CRS. The model GeoJSON is the project's
// stored model and is in projectCRS; the receiver table, the raster and the
// contours derived from it come out of a run and are in resultsCRS, which
// differs whenever the run had to project a geographic project CRS into a
// metric one before computing. Labelling each file with the CRS it is actually
// in is what lets a GIS overlay them; one shared label would be wrong for one
// of the two.
type formatExportContext struct {
	bundleDir        string
	formatsDir       string
	projectCRS       string
	epsgCode         int
	resultsCRS       string
	resultsEPSG      int
	contourInterval  float64
	modelGeoJSONPath string
	receiverTable    *results.ReceiverTable
	raster           *results.Raster
	geoTransform     exportfmt.GeoTransform
	hasGeoTransform  bool
	// Why a run result could not be read, held rather than raised.
	//
	// Held, because a format that never touches the artifact must not fail over
	// it: `--format gpkg` reads the receiver table and the model, never the
	// raster, and a corrupt raster sidecar is no reason to refuse it a
	// GeoPackage it can write correctly. Each format asks for what it reads,
	// and gets the reason if that one thing is unreadable.
	//
	// Rather than raised, because the alternative was to drop these on the
	// floor — which left `ctx.raster` nil, which every raster format reads as
	// "this run computed none" and skips. `aconiq export --format geotiff` then
	// printed its success line over a bundle with no GeoTIFF in it.
	rasterErr        error
	receiverTableErr error
	// Why the declared georeference could not be used, where one was declared.
	// Held rather than swallowed: a declaration that will not convert is a
	// refusal, not a cue to infer.
	geoTransformErr error
}

func executeFormatExports(
	formats []exportfmt.Format,
	bundleDir string,
	projectCRS string,
	resultsCRS string,
	copiedResults copiedRunResults,
	contourInterval float64,
	modelGeoJSONPath string,
) (map[string][]string, error) {
	ctx := newFormatExportContext(bundleDir, projectCRS, resultsCRS, copiedResults, contourInterval, modelGeoJSONPath)

	out := make(map[string][]string)

	for _, f := range formats {
		err := ctx.exportFormat(f, out)
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}

// newFormatExportContext loads what the formats read, and records why anything
// it could not read is missing.
//
// It does not refuse here. The refusal belongs to the format that needs the
// artifact — see `rasterErr` on the struct — so a corrupt raster sidecar costs
// the GeoTIFF and the contours, not the GeoPackage that never opens it.
func newFormatExportContext(
	bundleDir string,
	projectCRS string,
	resultsCRS string,
	copiedResults copiedRunResults,
	contourInterval float64,
	modelGeoJSONPath string,
) formatExportContext {
	if strings.TrimSpace(resultsCRS) == "" {
		resultsCRS = projectCRS
	}

	ctx := formatExportContext{
		bundleDir:        bundleDir,
		formatsDir:       filepath.Join(bundleDir, "formats"),
		projectCRS:       projectCRS,
		resultsCRS:       resultsCRS,
		contourInterval:  contourInterval,
		modelGeoJSONPath: modelGeoJSONPath,
	}

	_, _ = fmt.Sscanf(projectCRS, "EPSG:%d", &ctx.epsgCode)
	_, _ = fmt.Sscanf(resultsCRS, "EPSG:%d", &ctx.resultsEPSG)

	// Load receiver table if available (needed for GeoPackage + geo-transform inference).
	if copiedResults.ReceiverTableJSON != "" {
		table, err := results.LoadReceiverTableJSON(copiedResults.ReceiverTableJSON)
		if err != nil {
			ctx.receiverTableErr = fmt.Errorf("read the run's receiver table for export: %w", err)
		} else {
			ctx.receiverTable = &table
		}
	}

	// Load raster if available (needed for GeoTIFF + contours).
	if len(copiedResults.RasterMetadataList) > 0 {
		r, err := results.LoadRaster(copiedResults.RasterMetadataList[0])
		if err != nil {
			ctx.rasterErr = fmt.Errorf("read the run's raster for export: %w", err)
		} else {
			ctx.raster = r
		}
	}

	ctx.resolveGeoTransform()

	return ctx
}

// rasterForExport returns the run's raster, the reason it is unreadable, or
// nil for a run that wrote none. The three are different answers and only the
// last one is a skip.
func (c *formatExportContext) rasterForExport() (*results.Raster, error) {
	return c.raster, c.rasterErr
}

// receiverTableForExport is the same contract for the receiver table.
func (c *formatExportContext) receiverTableForExport() (*results.ReceiverTable, error) {
	return c.receiverTable, c.receiverTableErr
}

// resolveGeoTransform settles where this run's raster sits on the ground.
//
// The sidecar's own georeference wins. It is what the run recorded, it
// distinguishes a grid from a scatter of receivers, and it needs no second
// file. Inference from the receiver table is the fallback for a run written
// before the sidecar carried one — it cannot tell those two cases apart, so it
// is never preferred where a declaration exists.
func (c *formatExportContext) resolveGeoTransform() {
	if c.raster == nil {
		return
	}

	meta := c.raster.Metadata()

	// A declaration is the answer or it is a refusal — never a reason to go
	// looking for a second opinion. Falling through to inference would answer
	// an unreadable row order with a guess, and a north-up raster guessed
	// south-up is a vertically mirrored noise map that looks entirely
	// plausible. `results.NewRaster` validates the georeference on load, so a
	// sidecar that reaches here with an invalid one cannot exist today; the
	// branch is what keeps "declared, not assumed" true of this function
	// rather than of a check three packages away.
	if meta.Geo != nil {
		declared, err := exportfmt.GeoTransformFromGeoreference(*meta.Geo, meta.Height)
		if err != nil {
			c.geoTransformErr = err

			return
		}

		c.geoTransform = declared
		c.hasGeoTransform = true

		return
	}

	// Only now does the receiver table matter to a raster format: without a
	// declaration, inference is the one thing left that can place the raster,
	// so a table that will not load is the reason a georeferenced format has to
	// refuse rather than a detail of a different export.
	if c.receiverTableErr != nil {
		c.geoTransformErr = c.receiverTableErr

		return
	}

	if c.receiverTable == nil {
		return
	}

	xs := make([]float64, 0, len(c.receiverTable.Records))
	ys := make([]float64, 0, len(c.receiverTable.Records))

	for _, r := range c.receiverTable.Records {
		xs = append(xs, r.X)
		ys = append(ys, r.Y)
	}

	if len(xs) != meta.Width*meta.Height {
		return
	}

	inferred, err := exportfmt.InferGeoTransformFromReceivers(xs, ys, meta.Width, meta.Height)
	if err == nil {
		c.geoTransform = inferred
		c.hasGeoTransform = true
	}
}

// errNoGeoTransform refuses a georeferenced format for a raster whose position
// on the ground is unknown.
//
// This used to be an identity transform — origin (0, height), one unit per
// pixel — written silently. A GeoTIFF carrying it opens in any GIS, sits off
// the coast of Africa, and says nothing about being wrong. A refusal naming
// the run is the only honest answer, and it is reachable: explicit receivers
// are not a grid, and no georeference describes them.
var errNoGeoTransform = errors.New(
	"the run's raster carries no georeference and none could be inferred from its receivers, " +
		"so a georeferenced format would have to invent one; re-run with --receiver-mode auto-grid, " +
		"or export a format that carries no coordinates",
)

// rasterGeoTransform returns the resolved transform, or refuses.
func (c *formatExportContext) rasterGeoTransform() (exportfmt.GeoTransform, error) {
	if c.hasGeoTransform {
		return c.geoTransform, nil
	}

	if c.geoTransformErr != nil {
		return exportfmt.GeoTransform{}, fmt.Errorf(
			"the run's raster declares a georeference this build cannot read, and a georeferenced format "+
				"would have to reinterpret it: %w", c.geoTransformErr,
		)
	}

	return exportfmt.GeoTransform{}, errNoGeoTransform
}

func (c *formatExportContext) exportFormat(f exportfmt.Format, out map[string][]string) error {
	switch f {
	case exportfmt.FormatGeoTIFF:
		return c.exportGeoTIFF(out)
	case exportfmt.FormatCOG:
		return c.exportCOG(out)
	case exportfmt.FormatGeoPackage:
		return c.exportGeoPackage(out)
	case exportfmt.FormatContourGeoJSON:
		return c.exportContourGeoJSON(out)
	case exportfmt.FormatContourGeoPackage:
		return c.exportContourGeoPackage(out)
	}

	return nil
}

func (c *formatExportContext) exportGeoTIFF(out map[string][]string) error {
	raster, err := c.rasterForExport()
	if err != nil {
		return fmt.Errorf("geotiff export: %w", err)
	}

	if raster == nil {
		return nil // skip if no raster available
	}

	geoTransform, err := c.rasterGeoTransform()
	if err != nil {
		return fmt.Errorf("geotiff export: %w", err)
	}

	basePath := filepath.Join(c.formatsDir, "raster")

	paths, err := exportfmt.ExportGeoTIFF(basePath, raster, geoTransform, c.resultsCRS)
	if err != nil {
		return fmt.Errorf("geotiff export: %w", err)
	}

	relPaths := make([]string, len(paths))
	for i, p := range paths {
		relPaths[i] = relativePath(c.bundleDir, p)
	}

	out[string(exportfmt.FormatGeoTIFF)] = relPaths

	return nil
}

func (c *formatExportContext) exportCOG(out map[string][]string) error {
	raster, err := c.rasterForExport()
	if err != nil {
		return fmt.Errorf("cog export: %w", err)
	}

	if raster == nil {
		return nil
	}

	geoTransform, err := c.rasterGeoTransform()
	if err != nil {
		return fmt.Errorf("cog export: %w", err)
	}

	cogBasePath := filepath.Join(c.formatsDir, "raster")

	cogPaths, err := exportfmt.ExportCOG(cogBasePath, raster, geoTransform, c.resultsCRS)
	if err != nil {
		return fmt.Errorf("cog export: %w", err)
	}

	cogRelPaths := make([]string, len(cogPaths))
	for i, p := range cogPaths {
		cogRelPaths[i] = relativePath(c.bundleDir, p)
	}

	out[string(exportfmt.FormatCOG)] = cogRelPaths

	return nil
}

func (c *formatExportContext) exportGeoPackage(out map[string][]string) error {
	var gpkgPaths []string

	receiverTable, err := c.receiverTableForExport()
	if err != nil {
		return fmt.Errorf("geopackage export: %w", err)
	}

	if receiverTable != nil {
		gpkgPath := filepath.Join(c.formatsDir, "receivers.gpkg")

		err := exportfmt.ExportReceiverGeoPackage(gpkgPath, *receiverTable, c.resultsCRS, c.resultsEPSG)
		if err != nil {
			return fmt.Errorf("geopackage export: %w", err)
		}

		gpkgPaths = append(gpkgPaths, relativePath(c.bundleDir, gpkgPath))
	}

	if c.modelGeoJSONPath != "" {
		// Refused rather than skipped, for the reason the two loads in
		// `newFormatExportContext` are. The path is only set once the model
		// GeoJSON has been copied into the bundle, so a load failure here is a
		// file that is present and will not parse — and skipping it writes a
		// bundle quietly missing `model.gpkg`, with one artifact ref where the
		// caller asked for two. An empty model is a different thing and stays
		// a skip: nothing to write is a legitimate answer.
		modelFeatures, loadErr := loadModelFeaturesFromGeoJSON(c.modelGeoJSONPath)
		if loadErr != nil {
			return fmt.Errorf("read the bundle's model GeoJSON for export: %w", loadErr)
		}

		if len(modelFeatures) > 0 {
			modelGpkgPath := filepath.Join(c.formatsDir, "model.gpkg")

			exportErr := exportfmt.ExportModelFeaturesGeoPackage(modelGpkgPath, modelFeatures, c.projectCRS, c.epsgCode)
			if exportErr != nil {
				return fmt.Errorf("model geopackage export: %w", exportErr)
			}

			gpkgPaths = append(gpkgPaths, relativePath(c.bundleDir, modelGpkgPath))
		}
	}

	if len(gpkgPaths) > 0 {
		out[string(exportfmt.FormatGeoPackage)] = gpkgPaths
	}

	return nil
}

// geoJSONCRS is the only CRS a GeoJSON file may be in. RFC 7946 §4 fixes it:
// a consumer reads a bare FeatureCollection as WGS84 whatever produced it.
const geoJSONCRS = "EPSG:4326"

// contoursInWGS84 moves contour vertices out of the CRS the results are in and
// into the one GeoJSON is defined in.
//
// The other formats in this bundle carry their CRS in their own metadata and
// so can stay in the results' CRS; GeoJSON cannot, which is why this format
// alone is reprojected rather than labelled.
//
// A results CRS with no EPSG code — a WKT: identifier — is left alone. There
// is nothing to transform through, and failing the export would break bundles
// that are produced today in exchange for nothing.
func contoursInWGS84(contours []exportfmt.ContourLine, resultsCRS string) ([]exportfmt.ContourLine, error) {
	if strings.EqualFold(strings.TrimSpace(resultsCRS), geoJSONCRS) {
		return contours, nil
	}

	from, ok := epsgCRS(resultsCRS)
	if !ok {
		return contours, nil
	}

	to, err := geo.ParseCRS(geoJSONCRS)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", geoJSONCRS, err)
	}

	pipeline, err := geo.BuildTransformPipeline(to, from)
	if err != nil {
		return nil, fmt.Errorf("build transform %s -> %s: %w", from.ID, geoJSONCRS, err)
	}

	out := make([]exportfmt.ContourLine, len(contours))

	for i, contour := range contours {
		moved := contour
		moved.Points = make([][2]float64, len(contour.Points))

		for j, point := range contour.Points {
			transformed, err := pipeline.ApplyPoint(geo.Point2D{X: point[0], Y: point[1]})
			if err != nil {
				return nil, fmt.Errorf("contour %d vertex %d: %w", i, j, err)
			}

			moved.Points[j] = [2]float64{transformed.X, transformed.Y}
		}

		out[i] = moved
	}

	return out, nil
}

func (c *formatExportContext) exportContourGeoJSON(out map[string][]string) error {
	raster, err := c.rasterForExport()
	if err != nil {
		return fmt.Errorf("contour generation: %w", err)
	}

	if raster == nil {
		return nil
	}

	geoTransform, err := c.rasterGeoTransform()
	if err != nil {
		return fmt.Errorf("contour generation: %w", err)
	}

	contours, err := exportfmt.GenerateContours(raster, geoTransform, exportfmt.ContourOptions{
		Interval: c.contourInterval,
	})
	if err != nil {
		return fmt.Errorf("contour generation: %w", err)
	}

	contours, err = contoursInWGS84(contours, c.resultsCRS)
	if err != nil {
		return fmt.Errorf("contour reprojection: %w", err)
	}

	contourPath := filepath.Join(c.formatsDir, "contours.geojson")

	err = exportfmt.ExportContourGeoJSON(contourPath, contours)
	if err != nil {
		return fmt.Errorf("contour geojson export: %w", err)
	}

	out[string(exportfmt.FormatContourGeoJSON)] = []string{relativePath(c.bundleDir, contourPath)}

	return nil
}

func (c *formatExportContext) exportContourGeoPackage(out map[string][]string) error {
	raster, err := c.rasterForExport()
	if err != nil {
		return fmt.Errorf("contour generation: %w", err)
	}

	if raster == nil {
		return nil
	}

	geoTransform, err := c.rasterGeoTransform()
	if err != nil {
		return fmt.Errorf("contour generation: %w", err)
	}

	contours, err := exportfmt.GenerateContours(raster, geoTransform, exportfmt.ContourOptions{
		Interval: c.contourInterval,
	})
	if err != nil {
		return fmt.Errorf("contour generation: %w", err)
	}

	contourGpkgPath := filepath.Join(c.formatsDir, "contours.gpkg")

	err = exportfmt.ExportContourGeoPackage(contourGpkgPath, contours, c.resultsCRS, c.resultsEPSG)
	if err != nil {
		return fmt.Errorf("contour geopackage export: %w", err)
	}

	out[string(exportfmt.FormatContourGeoPackage)] = []string{relativePath(c.bundleDir, contourGpkgPath)}

	return nil
}
