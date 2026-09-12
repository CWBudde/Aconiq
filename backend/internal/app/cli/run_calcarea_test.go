package cli

import (
	"encoding/csv"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
)

// The sources sit near the origin and the drawn area sits 10 km away and is
// smaller than they are. Nothing the two extents produce can be confused.
const (
	calcAreaTestMinX = 10000.0
	calcAreaTestMinY = 20000.0
	calcAreaTestMaxX = 10200.0
	calcAreaTestMaxY = 20200.0
)

const calcAreaTestModel = `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "src-1", "kind": "source", "source_type": "point"},
      "geometry": {"type": "Point", "coordinates": [0, 0]}
    },
    {
      "type": "Feature",
      "properties": {"id": "src-2", "kind": "source", "source_type": "point"},
      "geometry": {"type": "Point", "coordinates": [400, 400]}
    },
    {
      "type": "Feature",
      "properties": {"id": "calc-1", "kind": "calc-area"},
      "geometry": {"type": "Polygon", "coordinates": [[[10000,20000],[10200,20000],[10200,20200],[10000,20200],[10000,20000]]]}
    }
  ]
}`

const calcAreaTestModelWithoutArea = `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "src-1", "kind": "source", "source_type": "point"},
      "geometry": {"type": "Point", "coordinates": [0, 0]}
    },
    {
      "type": "Feature",
      "properties": {"id": "src-2", "kind": "source", "source_type": "point"},
      "geometry": {"type": "Point", "coordinates": [400, 400]}
    }
  ]
}`

func calcAreaTestSources() []freefield.Source {
	return []freefield.Source{
		{ID: "src-1", Point: geo.Point2D{X: 0, Y: 0}, EmissionDB: 100},
		{ID: "src-2", Point: geo.Point2D{X: 400, Y: 400}, EmissionDB: 100},
	}
}

func calcAreaTestOptions(paddingM float64) dummyRunOptions {
	return dummyRunOptions{
		GridResolutionM: 50,
		GridPaddingM:    paddingM,
		ReceiverHeightM: 4,
		SourceEmission:  100,
	}
}

// receiverBounds is the extent the generated receivers actually cover.
func receiverBounds(t *testing.T, receivers []geo.PointReceiver) geo.BBox {
	t.Helper()

	points := make([]geo.Point2D, 0, len(receivers))
	for _, receiver := range receivers {
		points = append(points, receiver.Point)
	}

	bbox, ok := geo.BBoxFromPoints(points)
	if !ok {
		t.Fatalf("receivers have no finite extent")
	}

	return bbox
}

func TestCalcAreaExtentReadsTheDrawnPolygon(t *testing.T) {
	t.Parallel()

	extent, err := calcAreaExtent(mustNormalizeModel(t, calcAreaTestModel))
	if err != nil {
		t.Fatalf("calc area extent: %v", err)
	}

	if extent == nil {
		t.Fatal("expected a calculation area extent")
	}

	if extent.MinX != calcAreaTestMinX || extent.MinY != calcAreaTestMinY ||
		extent.MaxX != calcAreaTestMaxX || extent.MaxY != calcAreaTestMaxY {
		t.Fatalf("unexpected calculation area extent: %#v", *extent)
	}
}

func TestCalcAreaExtentIsAbsentWithoutOne(t *testing.T) {
	t.Parallel()

	extent, err := calcAreaExtent(mustNormalizeModel(t, calcAreaTestModelWithoutArea))
	if err != nil {
		t.Fatalf("calc area extent: %v", err)
	}

	if extent != nil {
		t.Fatalf("expected no extent, got %#v", *extent)
	}
}

func TestAutoGridUsesTheCalcAreaInsteadOfTheSourceExtent(t *testing.T) {
	t.Parallel()

	extent, err := calcAreaExtent(mustNormalizeModel(t, calcAreaTestModel))
	if err != nil {
		t.Fatalf("calc area extent: %v", err)
	}

	receivers, _, _, err := buildDummyReceivers(calcAreaTestSources(), extent, calcAreaTestOptions(0))
	if err != nil {
		t.Fatalf("build receivers: %v", err)
	}

	bounds := receiverBounds(t, receivers)
	if bounds.MinX != calcAreaTestMinX || bounds.MinY != calcAreaTestMinY ||
		bounds.MaxX != calcAreaTestMaxX || bounds.MaxY != calcAreaTestMaxY {
		t.Fatalf("receivers do not cover the drawn area: %#v", bounds)
	}
}

func TestAutoGridFallsBackToTheSourceExtent(t *testing.T) {
	t.Parallel()

	receivers, _, _, err := buildDummyReceivers(calcAreaTestSources(), nil, calcAreaTestOptions(0))
	if err != nil {
		t.Fatalf("build receivers: %v", err)
	}

	bounds := receiverBounds(t, receivers)
	if bounds.MinX != 0 || bounds.MinY != 0 || bounds.MaxX != 400 || bounds.MaxY != 400 {
		t.Fatalf("receivers do not cover the source extent: %#v", bounds)
	}
}

// TestGridPaddingStillAppliesToTheCalcArea pins the parity decision: the
// browser kernel's buildReceiverGrid pads the calc-area bbox unconditionally,
// so the CLI does too, and grid_padding_m=0 is how a user asks for the drawn
// area exactly.
func TestGridPaddingStillAppliesToTheCalcArea(t *testing.T) {
	t.Parallel()

	extent, err := calcAreaExtent(mustNormalizeModel(t, calcAreaTestModel))
	if err != nil {
		t.Fatalf("calc area extent: %v", err)
	}

	const paddingM = 100.0

	unpadded, _, _, err := buildDummyReceivers(calcAreaTestSources(), extent, calcAreaTestOptions(0))
	if err != nil {
		t.Fatalf("build unpadded receivers: %v", err)
	}

	padded, _, _, err := buildDummyReceivers(calcAreaTestSources(), extent, calcAreaTestOptions(paddingM))
	if err != nil {
		t.Fatalf("build padded receivers: %v", err)
	}

	unpaddedBounds := receiverBounds(t, unpadded)

	paddedBounds := receiverBounds(t, padded)
	if paddedBounds.MinX != unpaddedBounds.MinX-paddingM || paddedBounds.MinY != unpaddedBounds.MinY-paddingM {
		t.Fatalf("padding was not applied to the drawn area: %#v vs %#v", paddedBounds, unpaddedBounds)
	}

	if paddedBounds.MaxX != unpaddedBounds.MaxX+paddingM || paddedBounds.MaxY != unpaddedBounds.MaxY+paddingM {
		t.Fatalf("padding was not applied to the drawn area: %#v vs %#v", paddedBounds, unpaddedBounds)
	}
}

// TestOversizedGridNamesTheExtentItCameFrom covers the refusal a user can now
// trigger by drawing, where before it took a pathological source extent.
func TestOversizedGridNamesTheExtentItCameFrom(t *testing.T) {
	t.Parallel()

	huge := geo.BBox{MinX: 0, MinY: 0, MaxX: 100000, MaxY: 100000}
	options := calcAreaTestOptions(0)
	options.GridResolutionM = 100

	cases := map[string]struct {
		calcArea *geo.BBox
		sources  []freefield.Source
		want     string
	}{
		"calculation area": {
			calcArea: &huge,
			sources:  calcAreaTestSources(),
			want:     "the model's calculation area",
		},
		"source extent": {
			calcArea: nil,
			sources: []freefield.Source{
				{ID: "src-1", Point: geo.Point2D{X: 0, Y: 0}, EmissionDB: 100},
				{ID: "src-2", Point: geo.Point2D{X: 100000, Y: 100000}, EmissionDB: 100},
			},
			want: "the source extent",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, _, err := buildDummyReceivers(testCase.sources, testCase.calcArea, options)
			if err == nil {
				t.Fatal("expected the over-cap refusal")
			}

			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("refusal does not name %q: %v", testCase.want, err)
			}

			// KindUserInput is what exits the CLI with code 2 and maps to a
			// 400 whose envelope the run dialog renders.
			var appErr *domainerrors.AppError
			if !errors.As(err, &appErr) || appErr.Kind != domainerrors.KindUserInput {
				t.Fatalf("expected a KindUserInput error, got %v", err)
			}
		})
	}
}

// A drawn calculation area puts an extent nobody could compute two gestures
// away. The refusal has to come from the grid's dimensions, because by the time
// Generate returns a slice to count, the memory is already gone: without the
// check this test does not fail, it exhausts the machine.
func TestOversizedGridIsRefusedBeforeItIsBuilt(t *testing.T) {
	t.Parallel()

	impossible := geo.BBox{MinX: 0, MinY: 0, MaxX: 1e9, MaxY: 1e9}
	options := calcAreaTestOptions(0)
	options.GridResolutionM = 1

	receivers, _, _, err := buildDummyReceivers(calcAreaTestSources(), &impossible, options)
	if err == nil {
		t.Fatal("expected the over-cap refusal")
	}

	if receivers != nil {
		t.Fatalf("a refused grid must hand back nothing, got %d receivers", len(receivers))
	}

	if !strings.Contains(err.Error(), "receiver grid too large") {
		t.Fatalf("unexpected refusal: %v", err)
	}

	// Reported from the count, not from a materialised slice, so it can name a
	// size no slice could reach. The trailing digits are lost to float64 at this
	// magnitude, which is the right trade for a number whose only job is to tell
	// the user their area is orders of magnitude too big.
	if !strings.Contains(err.Error(), "1000000002000000000") {
		t.Fatalf("the refusal should state the grid's full size: %v", err)
	}

	var appErr *domainerrors.AppError
	if !errors.As(err, &appErr) || appErr.Kind != domainerrors.KindUserInput {
		t.Fatalf("expected a KindUserInput error, got %v", err)
	}
}

// TestCustomReceiverModeIgnoresTheCalcArea: no grid is built in custom mode, so
// a model carrying both a drawn area and explicit receivers uses the receivers.
func TestCustomReceiverModeIgnoresTheCalcArea(t *testing.T) {
	t.Parallel()

	model := mustNormalizeModel(t, `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"id": "src-1", "kind": "source", "source_type": "point"},
      "geometry": {"type": "Point", "coordinates": [0, 0]}
    },
    {
      "type": "Feature",
      "properties": {"id": "rcv-1", "kind": "receiver", "height_m": 4},
      "geometry": {"type": "Point", "coordinates": [50, 50]}
    },
    {
      "type": "Feature",
      "properties": {"id": "calc-1", "kind": "calc-area"},
      "geometry": {"type": "Polygon", "coordinates": [[[10000,20000],[10200,20000],[10200,20200],[10000,20200],[10000,20000]]]}
    }
  ]
}`)

	gridBuilt := false

	receivers, _, _, calcArea, err := resolveGridReceivers(model, receiverModeCustom, func(*geo.BBox) ([]geo.PointReceiver, int, int, error) {
		gridBuilt = true

		return nil, 0, 0, nil
	})
	if err != nil {
		t.Fatalf("resolve receivers: %v", err)
	}

	if gridBuilt {
		t.Fatal("custom receiver mode must not build a grid")
	}

	if len(receivers) != 1 || receivers[0].ID != "rcv-1" {
		t.Fatalf("expected the explicit receiver, got %#v", receivers)
	}

	// The extent is still read, so the caller can log that a grid would have
	// used it — but nothing consumes it here.
	if calcArea == nil {
		t.Fatal("expected the calculation area to still be reported")
	}
}

func TestGridExtentLabel(t *testing.T) {
	t.Parallel()

	if got := gridExtentLabel(&geo.BBox{}); got != gridExtentCalcArea {
		t.Fatalf("expected %q, got %q", gridExtentCalcArea, got)
	}

	if got := gridExtentLabel(nil); got != gridExtentSource {
		t.Fatalf("expected %q, got %q", gridExtentSource, got)
	}
}

// TestRunHonoursTheDrawnCalculationArea is the end-to-end proof that this is
// not API-only: `aconiq init` -> `aconiq import` -> `aconiq run` lands the
// receivers on the drawn area and records which extent it used.
func TestRunHonoursTheDrawnCalculationArea(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	modelPath := filepath.Join(projectDir, "model.geojson")

	err := os.WriteFile(modelPath, []byte(calcAreaTestModel), 0o600)
	if err != nil {
		t.Fatalf("write model: %v", err)
	}

	mustRunCLI(t, "--project", projectDir, "init", "--name", "CalcArea", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--input", modelPath)
	mustRunCLI(t, "--project", projectDir, "run",
		"--standard", "dummy-freefield", "--experimental",
		"--param", "grid_resolution_m=50", "--param", "grid_padding_m=0")

	runDir := latestRunDir(t, projectDir)

	logPayload, err := os.ReadFile(filepath.Join(runDir, "run.log"))
	if err != nil {
		t.Fatalf("read run.log: %v", err)
	}

	if !strings.Contains(string(logPayload), "grid_extent="+gridExtentCalcArea) {
		t.Fatalf("run.log does not record the calculation area as the grid extent:\n%s", logPayload)
	}

	minX, minY, maxX, maxY := receiverTableBounds(t, filepath.Join(runDir, "results", "receivers.csv"))
	if minX != calcAreaTestMinX || minY != calcAreaTestMinY || maxX != calcAreaTestMaxX || maxY != calcAreaTestMaxY {
		t.Fatalf("receivers cover [%.1f,%.1f]x[%.1f,%.1f], not the drawn area", minX, maxX, minY, maxY)
	}
}

// latestRunDir returns the directory of the project's most recent run.
func latestRunDir(t *testing.T, projectDir string) string {
	t.Helper()

	runsDir := filepath.Join(projectDir, ".noise", "runs")

	entries, err := os.ReadDir(runsDir)
	if err != nil {
		t.Fatalf("read runs directory: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("project has no runs")
	}

	return filepath.Join(runsDir, entries[len(entries)-1].Name())
}

// receiverTableBounds reads the persisted receiver table's x/y extent.
func receiverTableBounds(t *testing.T, path string) (float64, float64, float64, float64) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open receiver table: %v", err)
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Fatalf("close receiver table: %v", closeErr)
		}
	}()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read receiver table: %v", err)
	}

	if len(rows) < 2 {
		t.Fatalf("receiver table has no rows")
	}

	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)

	for _, row := range rows[1:] {
		x, parseErr := strconv.ParseFloat(row[1], 64)
		if parseErr != nil {
			t.Fatalf("parse receiver x %q: %v", row[1], parseErr)
		}

		y, parseErr := strconv.ParseFloat(row[2], 64)
		if parseErr != nil {
			t.Fatalf("parse receiver y %q: %v", row[2], parseErr)
		}

		minX, maxX = math.Min(minX, x), math.Max(maxX, x)
		minY, maxY = math.Min(minY, y), math.Max(maxY, y)
	}

	return minX, minY, maxX, maxY
}
