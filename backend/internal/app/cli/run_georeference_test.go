package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	exportfmt "github.com/aconiq/backend/internal/report/export"
	"github.com/aconiq/backend/internal/report/results"
)

// The raster sidecar is the only artifact that says where a run's cells sit on
// the ground. Before this it said nothing: origin and pixel size lived as loop
// state in geo.GridReceiverSet.Generate, were dropped on return, and every GIS
// export rebuilt them by arithmetic over the receiver table — with a silent
// identity transform whenever that arithmetic could not be trusted.
//
// The numbers asserted here are not free parameters. The fixture's sources span
// a known extent, grid_padding_m widens it on all four sides, and
// grid_resolution_m is the step, so the origin is the padded south-west corner
// and the pixel size is the resolution. Reading them back off the receiver
// table is what this test exists to make unnecessary, so it reads them off the
// receivers only to prove the sidecar agrees.
func TestRunWritesTheGridGeoreferenceIntoTheRasterSidecar(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Georeference", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, registryRunFixtures["rls19-road"]...))
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "rls19-road",
		"--param", "grid_resolution_m=25", "--param", "grid_padding_m=50")

	runID := digestRunRecord(t, projectDir, "rls19-road")
	resultsDir := filepath.Join(projectDir, ".noise", "runs", runID, "results")

	meta := readRasterSidecar(t, filepath.Join(resultsDir, "rls19-road.json"))

	if meta.CRS != "EPSG:25832" {
		t.Fatalf("sidecar crs = %q, want the compute CRS EPSG:25832", meta.CRS)
	}

	if meta.Geo == nil {
		t.Fatal("an auto-grid run wrote no georeference")
	}

	if meta.Geo.PixelSizeM != 25 {
		t.Fatalf("pixel_size_m = %v, want the run's grid_resolution_m of 25", meta.Geo.PixelSizeM)
	}

	if meta.Geo.RowOrder != results.RowOrderSouthUp {
		t.Fatalf("row_order = %q, want %q", meta.Geo.RowOrder, results.RowOrderSouthUp)
	}

	// The origin is the centre of cell (0,0), which is the first receiver the
	// grid generated — Generate walks Y ascending from the padded MinY, so
	// receiver 0 is the south-west corner.
	table := readReceiverTableJSON(t, filepath.Join(resultsDir, "receivers.json"))
	if len(table.Records) == 0 {
		t.Fatal("run produced no receivers")
	}

	first := table.Records[0]
	if meta.Geo.OriginX != first.X || meta.Geo.OriginY != first.Y {
		t.Fatalf("origin (%v, %v) is not the first receiver (%v, %v)",
			meta.Geo.OriginX, meta.Geo.OriginY, first.X, first.Y)
	}

	// The step the sidecar declares has to be the step the receivers actually
	// walk, or a raster drawn from it lands beside its own values.
	if meta.Width < 2 {
		t.Fatalf("fixture produced a %d-column grid; this assertion needs at least two", meta.Width)
	}

	if second := table.Records[1]; second.X-first.X != meta.Geo.PixelSizeM {
		t.Fatalf("receiver step %v does not match the declared pixel_size_m %v",
			second.X-first.X, meta.Geo.PixelSizeM)
	}
}

// Explicit receivers are points a user placed, not a grid. An invented
// georeference here would be worse than none: every GIS export reads the
// sidecar's transform without a second opinion.
func TestRunWritesNoGeoreferenceForExplicitReceivers(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	mustRunCLI(t, "--project", projectDir, "init", "--name", "Georeference", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", testdataPath(t, "phase17", "rls19_parking_model.geojson"))
	mustRunCLI(t, "--project", projectDir, "run", "--standard", "rls19-road", "--receiver-mode", receiverModeCustom)

	runID := digestRunRecord(t, projectDir, "rls19-road")
	sidecar := filepath.Join(projectDir, ".noise", "runs", runID, "results", "rls19-road.json")

	_, err := os.Stat(sidecar)
	if os.IsNotExist(err) {
		return // no raster at all is also an honest answer for scattered receivers
	}

	meta := readRasterSidecar(t, sidecar)
	if meta.Geo != nil {
		t.Fatalf("explicit receivers were given a georeference: %+v", *meta.Geo)
	}
}

func readRasterSidecar(t *testing.T, path string) results.RasterMetadata {
	t.Helper()

	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raster sidecar %s: %v", path, err)
	}

	var meta results.RasterMetadata

	err = json.Unmarshal(encoded, &meta)
	if err != nil {
		t.Fatalf("decode raster sidecar %s: %v", path, err)
	}

	return meta
}

func readReceiverTableJSON(t *testing.T, path string) results.ReceiverTable {
	t.Helper()

	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read receiver table %s: %v", path, err)
	}

	var table results.ReceiverTable

	err = json.Unmarshal(encoded, &table)
	if err != nil {
		t.Fatalf("decode receiver table %s: %v", path, err)
	}

	return table
}

// The parity fixtures are all 2x2: two columns prove the X step and two rows
// prove that row 0 is the southernmost, which is everything these transforms
// can get wrong.
const (
	gridParityWidth  = 2
	gridParityHeight = 2
)

// The whole point of recording the georeference is that export stops guessing.
// This is the test that discriminates: the sidecar and the receiver table are
// made to disagree, and the resolved transform has to follow the sidecar.
// Before this change the sidecar was not read at all and inference would have
// won every time.
func TestFormatExportContextPrefersTheSidecarGeoreference(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Receivers on a 10 m grid at the origin — what inference would read.
	tablePath := filepath.Join(dir, "receivers.json")
	writeGridReceiverTable(t, tablePath, 0, 0, 10)

	// The sidecar says somewhere else entirely, on a different step.
	rasterPath := writeGridRaster(t, filepath.Join(dir, "grid"), &results.Georeference{
		OriginX: 700000, OriginY: 5700000, PixelSizeM: 25, RowOrder: results.RowOrderSouthUp,
	})

	ctx := mustFormatExportContext(t, dir, "EPSG:25832", "EPSG:25832", copiedRunResults{
		ReceiverTableJSON:  tablePath,
		RasterMetadataList: []string{rasterPath},
	})

	got, err := ctx.rasterGeoTransform()
	if err != nil {
		t.Fatalf("resolve geo transform: %v", err)
	}

	want := exportfmt.GeoTransform{OriginX: 699987.5, OriginY: 5700037.5, PixelSizeX: 25, PixelSizeY: -25}
	if got != want {
		t.Fatalf("got %+v, want the sidecar's %+v", got, want)
	}
}

// A sidecar written before the georeference existed still has to export. The
// receiver table is the fallback, and it must still be consulted.
func TestFormatExportContextFallsBackToInferenceForAnOlderSidecar(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	tablePath := filepath.Join(dir, "receivers.json")
	writeGridReceiverTable(t, tablePath, 700000, 5700000, 25)

	rasterPath := writeGridRaster(t, filepath.Join(dir, "grid"), nil)

	ctx := mustFormatExportContext(t, dir, "EPSG:25832", "EPSG:25832", copiedRunResults{
		ReceiverTableJSON:  tablePath,
		RasterMetadataList: []string{rasterPath},
	})

	got, err := ctx.rasterGeoTransform()
	if err != nil {
		t.Fatalf("resolve geo transform: %v", err)
	}

	want := exportfmt.GeoTransform{OriginX: 699987.5, OriginY: 5700037.5, PixelSizeX: 25, PixelSizeY: -25}
	if got != want {
		t.Fatalf("got %+v, want the inferred %+v", got, want)
	}
}

// With neither a declaration nor receivers to infer from, the answer is a
// refusal. It used to be an identity transform written in silence: a GeoTIFF
// that opens anywhere, sits off the coast of Africa, and never says so.
func TestFormatExportContextRefusesToInventATransform(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rasterPath := writeGridRaster(t, filepath.Join(dir, "grid"), nil)

	ctx := mustFormatExportContext(t, dir, "EPSG:25832", "EPSG:25832", copiedRunResults{
		RasterMetadataList: []string{rasterPath},
	})

	_, err := ctx.rasterGeoTransform()
	if !errors.Is(err, errNoGeoTransform) {
		t.Fatalf("got %v, want errNoGeoTransform", err)
	}
}

// A georeferenced format must carry the refusal out to the user rather than
// writing a file at the origin.
func TestExportRefusesAGeoreferencedFormatWithoutATransform(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rasterPath := writeGridRaster(t, filepath.Join(dir, "grid"), nil)

	ctx := mustFormatExportContext(t, dir, "EPSG:25832", "EPSG:25832", copiedRunResults{
		RasterMetadataList: []string{rasterPath},
	})

	for name, export := range map[string]func(map[string][]string) error{
		"geotiff":         ctx.exportGeoTIFF,
		"cog":             ctx.exportCOG,
		"contour-geojson": ctx.exportContourGeoJSON,
		"contour-gpkg":    ctx.exportContourGeoPackage,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := export(map[string][]string{})
			if !errors.Is(err, errNoGeoTransform) {
				t.Fatalf("%s: got %v, want errNoGeoTransform", name, err)
			}
		})
	}
}

// A raster sidecar that will not load is a refusal, not an absent raster —
// but only for the formats that read one.
//
// The load error used to be dropped, which left the context without a raster,
// and every raster format reads a missing raster as "this run computed none"
// and skips it: `--format geotiff` reported success over a bundle with no
// GeoTIFF in it. Raising it from the constructor instead over-corrected, and
// took `--format gpkg` down with it — the GeoPackage is built from the
// receiver table and the model and never opens the raster at all.
func TestAnUnreadableRasterRefusesOnlyTheFormatsThatReadIt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rasterPath := writeGridRaster(t, filepath.Join(dir, "grid"), &results.Georeference{
		OriginX: 700000, OriginY: 5700000, PixelSizeM: 25, RowOrder: results.RowOrderSouthUp,
	})

	tablePath := filepath.Join(dir, "receivers.json")
	writeGridReceiverTable(t, tablePath, 700000, 5700000, 25)

	err := os.WriteFile(rasterPath, []byte("{not json"), 0o600)
	if err != nil {
		t.Fatalf("corrupt the sidecar: %v", err)
	}

	ctx := mustFormatExportContext(t, dir, "EPSG:25832", "EPSG:25832", copiedRunResults{
		ReceiverTableJSON:  tablePath,
		RasterMetadataList: []string{rasterPath},
	})

	for name, export := range map[string]func(map[string][]string) error{
		"geotiff":         ctx.exportGeoTIFF,
		"cog":             ctx.exportCOG,
		"contour-geojson": ctx.exportContourGeoJSON,
		"contour-gpkg":    ctx.exportContourGeoPackage,
	} {
		err := export(map[string][]string{})
		if err == nil {
			t.Fatalf("%s exported as though the run had written no raster", name)
		}
	}

	out := map[string][]string{}

	err = ctx.exportGeoPackage(out)
	if err != nil {
		t.Fatalf("gpkg refused over a raster it never reads: %v", err)
	}

	if len(out[string(exportfmt.FormatGeoPackage)]) == 0 {
		t.Fatal("gpkg wrote nothing")
	}
}

// The same for the receiver table, in the other direction: GeoPackage is built
// from it and must refuse, while a raster carrying its own georeference needs
// no receivers and must still export.
func TestAnUnreadableReceiverTableRefusesOnlyTheFormatsThatReadIt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rasterPath := writeGridRaster(t, filepath.Join(dir, "grid"), &results.Georeference{
		OriginX: 700000, OriginY: 5700000, PixelSizeM: 25, RowOrder: results.RowOrderSouthUp,
	})

	tablePath := filepath.Join(dir, "receivers.json")

	err := os.WriteFile(tablePath, []byte("{not json"), 0o600)
	if err != nil {
		t.Fatalf("write the table: %v", err)
	}

	ctx := mustFormatExportContext(t, dir, "EPSG:25832", "EPSG:25832", copiedRunResults{
		ReceiverTableJSON:  tablePath,
		RasterMetadataList: []string{rasterPath},
	})

	err = ctx.exportGeoPackage(map[string][]string{})
	if err == nil {
		t.Fatal("an unreadable receiver table exported as though the run had none")
	}

	out := map[string][]string{}

	err = ctx.exportGeoTIFF(out)
	if err != nil {
		t.Fatalf("geotiff refused over a receiver table its sidecar makes unnecessary: %v", err)
	}

	if len(out[string(exportfmt.FormatGeoTIFF)]) == 0 {
		t.Fatal("geotiff wrote nothing")
	}
}

// Where the sidecar carries no georeference, inference is the only thing left
// that can place the raster — so there the unreadable table *is* the raster
// formats' problem, and they have to say so rather than skip.
func TestAnUnreadableReceiverTableRefusesARasterThatNeedsInference(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rasterPath := writeGridRaster(t, filepath.Join(dir, "grid"), nil)

	tablePath := filepath.Join(dir, "receivers.json")

	err := os.WriteFile(tablePath, []byte("{not json"), 0o600)
	if err != nil {
		t.Fatalf("write the table: %v", err)
	}

	ctx := mustFormatExportContext(t, dir, "EPSG:25832", "EPSG:25832", copiedRunResults{
		ReceiverTableJSON:  tablePath,
		RasterMetadataList: []string{rasterPath},
	})

	err = ctx.exportGeoTIFF(map[string][]string{})
	if err == nil {
		t.Fatal("geotiff wrote a raster it had no way to place")
	}

	if errors.Is(err, errNoGeoTransform) {
		t.Fatalf("the refusal blames a missing georeference, not the table it could not read: %v", err)
	}
}

// And the same for the bundle's model GeoJSON, which GeoPackage reads. The
// path is only set once the file is in the bundle, so a load failure is a file
// that is there and will not parse — skipping it writes a bundle missing
// `model.gpkg` and reports one artifact where the caller asked for two.
func TestExportGeoPackageRefusesAnUnreadableModelGeoJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	modelPath := filepath.Join(dir, "model.geojson")

	err := os.WriteFile(modelPath, []byte("{not json"), 0o600)
	if err != nil {
		t.Fatalf("write the model: %v", err)
	}

	ctx := newFormatExportContext(dir, "EPSG:25832", "EPSG:25832", copiedRunResults{}, 5.0, modelPath)

	err = ctx.exportGeoPackage(map[string][]string{})
	if err == nil {
		t.Fatal("an unreadable model GeoJSON exported as though the bundle carried none")
	}
}

// A declared georeference that will not convert is a refusal, and the refusal
// must name the declaration rather than reading as "no georeference" — the
// latter is what sends `resolveGeoTransform` to inference.
//
// The state is built directly because no stored raster can reach it:
// `results.NewRaster` validates the georeference on load, so a sidecar
// carrying an unreadable row order never becomes a raster in the first place.
// The branch is what keeps "declared, not assumed" a property of this
// resolver instead of a check three packages away.
func TestRasterGeoTransformRefusalNamesABadDeclaration(t *testing.T) {
	t.Parallel()

	ctx := formatExportContext{
		geoTransformErr: errors.New("georeference row_order \"north-up\" is not supported"),
	}

	_, err := ctx.rasterGeoTransform()
	if err == nil {
		t.Fatal("an unreadable row order resolved to a transform")
	}

	if errors.Is(err, errNoGeoTransform) {
		t.Fatalf("got the no-georeference refusal, which is the one that licenses inference: %v", err)
	}

	if !strings.Contains(err.Error(), "north-up") {
		t.Fatalf("the refusal does not say what it could not read: %v", err)
	}
}

func mustFormatExportContext(
	t *testing.T,
	bundleDir string,
	projectCRS string,
	resultsCRS string,
	copiedResults copiedRunResults,
) formatExportContext {
	t.Helper()

	return newFormatExportContext(bundleDir, projectCRS, resultsCRS, copiedResults, 5.0, "")
}

// writeGridReceiverTable writes a row-major, Y-ascending receiver table — the
// order geo.GridReceiverSet.Generate produces and the one inference expects.
func writeGridReceiverTable(t *testing.T, path string, originX, originY, step float64) {
	t.Helper()

	table := results.ReceiverTable{
		IndicatorOrder: []string{"Lden"},
		Unit:           "dB",
		Records:        make([]results.ReceiverRecord, 0, gridParityWidth*gridParityHeight),
	}

	index := 0

	for row := range gridParityHeight {
		for col := range gridParityWidth {
			table.Records = append(table.Records, results.ReceiverRecord{
				ID:      fmt.Sprintf("grid-%06d", index),
				X:       originX + float64(col)*step,
				Y:       originY + float64(row)*step,
				HeightM: 4,
				Values:  map[string]float64{"Lden": 55},
			})
			index++
		}
	}

	err := results.SaveReceiverTableJSON(path, table)
	if err != nil {
		t.Fatalf("save receiver table: %v", err)
	}
}

func writeGridRaster(t *testing.T, basePath string, georef *results.Georeference) string {
	t.Helper()

	raster, err := results.NewRaster(results.RasterMetadata{
		Width: gridParityWidth, Height: gridParityHeight, Bands: 1, NoData: -9999, Unit: "dB",
		BandNames: []string{"Lden"}, CRS: "EPSG:25832", Geo: georef,
	})
	if err != nil {
		t.Fatalf("new raster: %v", err)
	}

	raster.Fill(55)

	persistence, err := results.SaveRaster(basePath, raster)
	if err != nil {
		t.Fatalf("save raster: %v", err)
	}

	return persistence.MetadataPath
}
