// The SoundPLAN format of `aconiq export --format soundplan-shp`.
//
// Split out of export_formats.go because it is the one format that exports the
// model rather than a run's results: it reads the bundle's model GeoJSON and
// the project's DTM, and the run contributes only the CRS it computed in.

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/geo/terrain"
	"github.com/aconiq/backend/internal/jsonio"
	exportfmt "github.com/aconiq/backend/internal/report/export"
)

// modelExportInputs is what the model formats read beyond the bundle.
type modelExportInputs struct {
	// modelCRS is the CRS the bundle's model GeoJSON is in: --target-crs
	// when the export reprojected it, the project CRS otherwise.
	modelCRS string
	// projectCRS is the CRS the project's DTM is stored in.
	projectCRS string
	// terrainPath is the absolute path of the project's DTM, empty when the
	// project has none.
	terrainPath string
	// date is stamped into the .dbf headers: the run's finish time, so that
	// exporting one run twice writes the same layer bytes.
	date time.Time
}

// newModelExportInputs records what the model formats need from the project
// and the run. targetCRS is --target-crs, which reprojects the bundle's model
// GeoJSON but not the project's DTM.
func newModelExportInputs(storeRoot string, proj project.Project, run project.Run, targetCRS string) modelExportInputs {
	inputs := modelExportInputs{
		modelCRS:   proj.CRS,
		projectCRS: proj.CRS,
		date:       run.FinishedAt,
	}

	if targetCRS != "" {
		inputs.modelCRS = targetCRS
	}

	if terrainPath := findArtifactPath(proj, project.ArtifactIDTerrain); terrainPath != "" {
		inputs.terrainPath = filepath.Join(storeRoot, filepath.FromSlash(terrainPath))
	}

	return inputs
}

// soundPLANDir is the bundle-relative directory the SoundPLAN files go into.
const soundPLANDir = "soundplan"

func (c *formatExportContext) exportSoundPLAN(out map[string][]string) error {
	// Refused rather than skipped: this format is the model, so a bundle
	// without one cannot answer it, and an empty success would read as a
	// project with nothing in it.
	if c.modelGeoJSONPath == "" {
		return domainerrors.New(domainerrors.KindUserInput, "cli.export",
			"soundplan export: the project has no imported model to export; run `aconiq import` first", nil)
	}

	// SoundPLAN gets the coordinates the run computed in. For a projected
	// project that is the project CRS; for a geographic one it is the UTM
	// zone the run projected into, because SoundPLAN measures distances in
	// the plane of whatever it imports.
	targetCRS := c.resultsCRS

	features, sourceCRS, err := c.soundPLANFeatures(targetCRS)
	if err != nil {
		return err
	}

	dir := filepath.Join(c.formatsDir, soundPLANDir)

	report, paths, err := exportfmt.ExportSoundPLANShapefiles(dir, features, exportfmt.SoundPLANOptions{
		CRS:       targetCRS,
		SourceCRS: sourceCRS,
		Date:      c.model.date,
	})
	if errors.Is(err, exportfmt.ErrSoundPLANCRS) {
		// The run's CRS is the user's choice, and computing in a metric
		// conformal one is how they meet it: exit 2, not 1.
		return domainerrors.New(domainerrors.KindUserInput, "cli.export", "", err)
	}

	if err != nil {
		return fmt.Errorf("soundplan export: %w", err)
	}

	if c.model.terrainPath != "" {
		terrainPath := filepath.Join(dir, exportfmt.SoundPLANTerrainFile)

		terrainReport, err := writeSoundPLANTerrain(terrainPath, c.model.terrainPath, c.model.projectCRS, targetCRS)
		if err != nil {
			return fmt.Errorf("soundplan export: %w", err)
		}

		report.Terrain = &terrainReport

		paths = append(paths, terrainPath)
	}

	reportPath := filepath.Join(dir, exportfmt.SoundPLANReportFile)

	err = report.Write(reportPath)
	if err != nil {
		return fmt.Errorf("soundplan export: %w", err)
	}

	paths = append(paths, reportPath)

	relPaths := make([]string, len(paths))
	for i, p := range paths {
		relPaths[i] = relativePath(c.bundleDir, p)
	}

	out[string(exportfmt.FormatSoundPLANShapefile)] = relPaths

	return nil
}

// soundPLANFeatures reads the bundle's model in targetCRS. It returns the CRS
// it was projected from, or "" when it was already there.
func (c *formatExportContext) soundPLANFeatures(targetCRS string) ([]exportfmt.ModelFeature, string, error) {
	data, err := os.ReadFile(c.modelGeoJSONPath)
	if err != nil {
		return nil, "", fmt.Errorf("soundplan export: read the bundle's model GeoJSON: %w", err)
	}

	same, err := sameCRS(c.model.modelCRS, targetCRS)
	if err != nil {
		return nil, "", fmt.Errorf("soundplan export: %w", err)
	}

	if same {
		features, err := modelFeaturesFromGeoJSON(data)
		if err != nil {
			return nil, "", fmt.Errorf("soundplan export: %w", err)
		}

		return features, "", nil
	}

	// The reprojection `--target-crs` and `aconiq run` both go through, so
	// the model reaches SoundPLAN where the run placed it.
	model, err := modelgeojson.NormalizeWithCRS(data, targetCRS, c.model.modelCRS, "export")
	if err != nil {
		return nil, "", fmt.Errorf("soundplan export: project the model %s -> %s: %w", c.model.modelCRS, targetCRS, err)
	}

	projected, err := jsonio.Marshal(model.ToFeatureCollection())
	if err != nil {
		return nil, "", fmt.Errorf("soundplan export: encode the projected model: %w", err)
	}

	features, err := modelFeaturesFromGeoJSON(projected)
	if err != nil {
		return nil, "", fmt.Errorf("soundplan export: %w", err)
	}

	return features, c.model.modelCRS, nil
}

// sameCRS compares two CRS strings by what they name rather than how they are
// spelled; two empty strings are the same undeclared CRS.
func sameCRS(a, b string) (bool, error) {
	if a == b {
		return true, nil
	}

	pa, err := geo.ParseCRS(a)
	if err != nil {
		return false, fmt.Errorf("read the CRS %q: %w", a, err)
	}

	pb, err := geo.ParseCRS(b)
	if err != nil {
		return false, fmt.Errorf("read the CRS %q: %w", b, err)
	}

	return pa.ID == pb.ID, nil
}

// writeSoundPLANTerrain writes the project's DTM as elevation points in
// targetCRS. The DTM is stored in the project CRS; when the run computed
// elsewhere, every cell centre is projected, so the points stay exact samples
// of the DTM but no longer sit on a regular grid.
func writeSoundPLANTerrain(path, dtmPath, dtmCRS, targetCRS string) (exportfmt.SoundPLANTerrainReport, error) {
	model, err := terrain.Load(dtmPath)
	if err != nil {
		return exportfmt.SoundPLANTerrainReport{}, fmt.Errorf("read the project DTM: %w", err)
	}

	project, err := cellProjection(dtmCRS, targetCRS)
	if err != nil {
		return exportfmt.SoundPLANTerrainReport{}, fmt.Errorf("terrain: %w", err)
	}

	report, err := exportfmt.WriteSoundPLANTerrain(path, func(emit func(x, y, z float64) error) error {
		return terrain.EachCell(model, func(x, y, z float64) error {
			p, err := project(geo.Point2D{X: x, Y: y})
			if err != nil {
				return fmt.Errorf("terrain: project cell (%v, %v): %w", x, y, err)
			}

			return emit(p.X, p.Y, z)
		})
	})
	if err != nil {
		return exportfmt.SoundPLANTerrainReport{}, fmt.Errorf("terrain: %w", err)
	}

	return report, nil
}

// cellProjection maps a DTM cell centre from the DTM's CRS into targetCRS.
func cellProjection(dtmCRS, targetCRS string) (func(geo.Point2D) (geo.Point2D, error), error) {
	same, err := sameCRS(dtmCRS, targetCRS)
	if err != nil {
		return nil, err
	}

	if same {
		return func(p geo.Point2D) (geo.Point2D, error) { return p, nil }, nil
	}

	target, err := geo.ParseCRS(targetCRS)
	if err != nil {
		return nil, fmt.Errorf("read the CRS %q: %w", targetCRS, err)
	}

	native, err := geo.ParseCRS(dtmCRS)
	if err != nil {
		return nil, fmt.Errorf("read the CRS %q: %w", dtmCRS, err)
	}

	// BuildTransformPipeline maps its second argument into its first.
	pipeline, err := geo.BuildTransformPipeline(target, native)
	if err != nil {
		return nil, fmt.Errorf("build transform %s -> %s: %w", native.ID, target.ID, err)
	}

	return pipeline.ApplyPoint, nil
}
