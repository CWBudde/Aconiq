package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

	ctx := newFormatExportContext(dir, "EPSG:25832", "EPSG:25832", copiedRunResults{
		ReceiverTableJSON:  tablePath,
		RasterMetadataList: []string{rasterPath},
	}, 5.0, "")

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

	ctx := newFormatExportContext(dir, "EPSG:25832", "EPSG:25832", copiedRunResults{
		ReceiverTableJSON:  tablePath,
		RasterMetadataList: []string{rasterPath},
	}, 5.0, "")

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

	ctx := newFormatExportContext(dir, "EPSG:25832", "EPSG:25832", copiedRunResults{
		RasterMetadataList: []string{rasterPath},
	}, 5.0, "")

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

	ctx := newFormatExportContext(dir, "EPSG:25832", "EPSG:25832", copiedRunResults{
		RasterMetadataList: []string{rasterPath},
	}, 5.0, "")

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
