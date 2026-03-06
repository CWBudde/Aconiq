package industry

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/qa/golden"
	"github.com/aconiq/backend/internal/report/results"
)

func TestIndustrySourceValidate(t *testing.T) {
	t.Parallel()

	source := samplePointSource()

	err := source.Validate()
	if err != nil {
		t.Fatalf("valid source failed validation: %v", err)
	}

	source.SourceHeightM = -1

	err = source.Validate()
	if err == nil {
		t.Fatal("expected invalid source height")
	}

	source = samplePointSource()
	source.SourceCategory = "unknown"

	err = source.Validate()
	if err == nil {
		t.Fatal("expected invalid source category")
	}
}

func TestAreaEmissionIncreasesWithArea(t *testing.T) {
	t.Parallel()

	small := sampleAreaSource()
	small.AreaPolygon = [][]geo.Point2D{
		{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}, {X: 0, Y: 0}},
	}

	large := sampleAreaSource()
	large.AreaPolygon = [][]geo.Point2D{
		{{X: 0, Y: 0}, {X: 30, Y: 0}, {X: 30, Y: 30}, {X: 0, Y: 30}, {X: 0, Y: 0}},
	}

	smallEmission, err := ComputeEmission(small)
	if err != nil {
		t.Fatalf("compute small emission: %v", err)
	}

	largeEmission, err := ComputeEmission(large)
	if err != nil {
		t.Fatalf("compute large emission: %v", err)
	}

	if largeEmission.Lday <= smallEmission.Lday {
		t.Fatalf("expected larger area to increase Lday: small=%f large=%f", smallEmission.Lday, largeEmission.Lday)
	}
}

func TestEmissionUsesCategoryAndEnclosure(t *testing.T) {
	t.Parallel()

	open := samplePointSource()
	open.SourceCategory = CategoryProcess
	open.EnclosureState = EnclosureOpen

	openEmission, err := ComputeEmission(open)
	if err != nil {
		t.Fatalf("compute open emission: %v", err)
	}

	enclosed := samplePointSource()
	enclosed.SourceCategory = CategoryStack
	enclosed.EnclosureState = EnclosureEnclosed

	enclosedEmission, err := ComputeEmission(enclosed)
	if err != nil {
		t.Fatalf("compute enclosed emission: %v", err)
	}

	if enclosedEmission.Lday >= openEmission.Lday {
		t.Fatalf("expected enclosed source to reduce Lday: open=%f enclosed=%f", openEmission.Lday, enclosedEmission.Lday)
	}
}

func TestPropagationDecreasesWithDistance(t *testing.T) {
	t.Parallel()

	source := samplePointSource()
	cfg := DefaultPropagationConfig()

	near, err := ComputeReceiverPeriodLevels(geo.PointReceiver{ID: "near", Point: geo.Point2D{X: 5, Y: 0}, HeightM: 4}, []IndustrySource{source}, cfg)
	if err != nil {
		t.Fatalf("compute near receiver: %v", err)
	}

	far, err := ComputeReceiverPeriodLevels(geo.PointReceiver{ID: "far", Point: geo.Point2D{X: 250, Y: 0}, HeightM: 4}, []IndustrySource{source}, cfg)
	if err != nil {
		t.Fatalf("compute far receiver: %v", err)
	}

	if near.Lday <= far.Lday {
		t.Fatalf("expected near level > far level: near=%f far=%f", near.Lday, far.Lday)
	}
}

func TestAttenuationTermsExposePropagationComponents(t *testing.T) {
	t.Parallel()

	receiver := geo.PointReceiver{ID: "r1", Point: geo.Point2D{X: 10, Y: 0}, HeightM: 4}
	source := samplePointSource()
	cfg := DefaultPropagationConfig()
	terms := attenuationTerms(receiver, source, cfg)

	if terms.DistanceM <= 0 {
		t.Fatalf("unexpected distance term: %#v", terms)
	}

	if math.Abs(terms.AirDB-airAbsorption(terms.DistanceM, cfg)) > 1e-9 {
		t.Fatalf("unexpected air term: %#v", terms)
	}
}

func TestAreaGeometryEffectIncreasesWithArea(t *testing.T) {
	t.Parallel()

	receiver := geo.PointReceiver{ID: "r1", Point: geo.Point2D{X: 60, Y: 0}, HeightM: 4}
	cfg := DefaultPropagationConfig()

	small := sampleAreaSource()
	small.AreaPolygon = [][]geo.Point2D{
		{{X: 20, Y: -10}, {X: 30, Y: -10}, {X: 30, Y: 0}, {X: 20, Y: 0}, {X: 20, Y: -10}},
	}

	large := sampleAreaSource()
	large.AreaPolygon = [][]geo.Point2D{
		{{X: 20, Y: -20}, {X: 60, Y: -20}, {X: 60, Y: 20}, {X: 20, Y: 20}, {X: 20, Y: -20}},
	}

	if areaGeometryEffect(receiver, large, cfg) <= areaGeometryEffect(receiver, small, cfg) {
		t.Fatalf("expected larger area geometry effect to increase")
	}
}

func TestIndustryLdenAggregation(t *testing.T) {
	t.Parallel()

	levels := PeriodLevels{
		Lday:     57,
		Levening: 57,
		Lnight:   57,
	}
	lden := ComputeLden(levels)

	expected := 63.39524300131856
	if math.Abs(lden-expected) > 1e-9 {
		t.Fatalf("unexpected Lden: got %.12f expected %.12f", lden, expected)
	}
}

func TestIndustryExportResultBundle(t *testing.T) {
	t.Parallel()

	receivers := []geo.PointReceiver{
		{ID: "r1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4},
		{ID: "r2", Point: geo.Point2D{X: 10, Y: 0}, HeightM: 4},
		{ID: "r3", Point: geo.Point2D{X: 0, Y: 10}, HeightM: 4},
		{ID: "r4", Point: geo.Point2D{X: 10, Y: 10}, HeightM: 4},
	}

	outputs, err := ComputeReceiverOutputs(receivers, []IndustrySource{samplePointSource(), sampleAreaSource()}, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute receiver outputs: %v", err)
	}

	dir := t.TempDir()

	exported, err := ExportResultBundle(dir, outputs, 2, 2)
	if err != nil {
		t.Fatalf("export result bundle: %v", err)
	}

	for _, path := range []string{exported.ReceiverJSONPath, exported.ReceiverCSVPath, exported.RasterMetaPath, exported.RasterDataPath} {
		_, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected exported file %s: %v", path, err)
		}
	}

	raster, err := results.LoadRaster(exported.RasterMetaPath)
	if err != nil {
		t.Fatalf("load raster: %v", err)
	}

	if raster.Metadata().Bands != 2 {
		t.Fatalf("expected 2 raster bands")
	}
}

func TestIndustryGoldenScenario(t *testing.T) {
	t.Parallel()

	var scenario struct {
		Sources    []IndustrySource    `json:"sources"`
		Receivers  []geo.PointReceiver `json:"receivers"`
		GridWidth  int                 `json:"grid_width"`
		GridHeight int                 `json:"grid_height"`
	}

	payload, err := os.ReadFile(testdataPath(t, "industry_scenario.json"))
	if err != nil {
		t.Fatalf("read industry scenario: %v", err)
	}

	err = json.Unmarshal(payload, &scenario)
	if err != nil {
		t.Fatalf("decode industry scenario: %v", err)
	}

	outputs, err := ComputeReceiverOutputs(scenario.Receivers, scenario.Sources, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute outputs: %v", err)
	}

	snapshot := map[string]any{
		"receiver_count": len(outputs),
		"grid_width":     scenario.GridWidth,
		"grid_height":    scenario.GridHeight,
		"receivers":      roundedOutputs(outputs),
	}

	golden.AssertJSONSnapshot(t, testdataPath(t, "industry_scenario.golden.json"), snapshot)
}

func TestIndustryProvenanceMetadataIncludesExpandedKeyParameters(t *testing.T) {
	t.Parallel()

	metadata := ProvenanceMetadata(map[string]string{
		"industry_source_category": "stack",
		"industry_enclosure_state": "partial",
		"operation_evening_factor": "0.5",
	})

	if metadata["model_version"] != BuiltinModelVersion {
		t.Fatalf("unexpected model_version: %#v", metadata)
	}

	if metadata["key_parameter.industry_source_category"] != "stack" {
		t.Fatalf("expected source category in provenance: %#v", metadata)
	}
}

func roundedOutputs(outputs []ReceiverOutput) []map[string]any {
	out := make([]map[string]any, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, map[string]any{
			"id":       output.Receiver.ID,
			"x":        round6(output.Receiver.Point.X),
			"y":        round6(output.Receiver.Point.Y),
			"height_m": round6(output.Receiver.HeightM),
			"Lday":     round6(output.Indicators.Lday),
			"Levening": round6(output.Indicators.Levening),
			"Lnight":   round6(output.Indicators.Lnight),
			"Lden":     round6(output.Indicators.Lden),
		})
	}

	return out
}

func round6(value float64) float64 {
	return math.Round(value*1e6) / 1e6
}

func testdataPath(t *testing.T, parts ...string) string {
	t.Helper()

	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve industry test file path")
	}

	base := filepath.Join(filepath.Dir(filePath), "testdata")
	all := append([]string{base}, parts...)

	return filepath.Join(all...)
}

func samplePointSource() IndustrySource {
	return IndustrySource{
		ID:                      "industry-point-1",
		SourceType:              SourceTypePoint,
		SourceCategory:          CategoryProcess,
		EnclosureState:          EnclosureOpen,
		Point:                   geo.Point2D{X: 0, Y: 0},
		SourceHeightM:           6,
		SoundPowerLevelDB:       98,
		TonalityCorrectionDB:    2,
		ImpulsivityCorrectionDB: 1,
		OperationDay:            OperationPeriod{OperatingFactor: 1.0},
		OperationEvening:        OperationPeriod{OperatingFactor: 0.8},
		OperationNight:          OperationPeriod{OperatingFactor: 0.5},
	}
}

func sampleAreaSource() IndustrySource {
	return IndustrySource{
		ID:             "industry-area-1",
		SourceType:     SourceTypeArea,
		SourceCategory: CategoryYard,
		EnclosureState: EnclosureOpen,
		AreaPolygon: [][]geo.Point2D{
			{{X: 20, Y: -10}, {X: 40, Y: -10}, {X: 40, Y: 10}, {X: 20, Y: 10}, {X: 20, Y: -10}},
		},
		SourceHeightM:           4,
		SoundPowerLevelDB:       82,
		TonalityCorrectionDB:    0,
		ImpulsivityCorrectionDB: 0,
		OperationDay:            OperationPeriod{OperatingFactor: 1.0},
		OperationEvening:        OperationPeriod{OperatingFactor: 0.6},
		OperationNight:          OperationPeriod{OperatingFactor: 0.3},
	}
}
