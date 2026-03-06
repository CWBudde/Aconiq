package aircraft

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

func TestAircraftSourceValidate(t *testing.T) {
	t.Parallel()

	source := sampleSource()

	err := source.Validate()
	if err != nil {
		t.Fatalf("valid source failed validation: %v", err)
	}

	source.EngineStateFactor = 0

	err = source.Validate()
	if err == nil {
		t.Fatal("expected invalid engine factor")
	}

	source = sampleSource()
	source.ProcedureType = "nope"

	err = source.Validate()
	if err == nil {
		t.Fatal("expected invalid procedure type")
	}
}

func TestEmissionIncreasesWithMovements(t *testing.T) {
	t.Parallel()

	low := sampleSource()
	low.MovementDay = MovementPeriod{MovementsPerHour: 2}

	high := sampleSource()
	high.MovementDay = MovementPeriod{MovementsPerHour: 12}

	lowEmission, err := ComputeEmission(low)
	if err != nil {
		t.Fatalf("compute low emission: %v", err)
	}

	highEmission, err := ComputeEmission(high)
	if err != nil {
		t.Fatalf("compute high emission: %v", err)
	}

	if highEmission.Lday <= lowEmission.Lday {
		t.Fatalf("expected higher movements to increase Lday: low=%f high=%f", lowEmission.Lday, highEmission.Lday)
	}
}

func TestEmissionUsesProcedureAndThrustContext(t *testing.T) {
	t.Parallel()

	base := sampleSource()
	contextual := sampleSource()
	contextual.AircraftClass = AircraftClassCargo
	contextual.ProcedureType = ProcedureContinuousDescent
	contextual.ThrustMode = ThrustIdle
	contextual.EngineStateFactor = 0.85

	baseEmission, err := ComputeEmission(base)
	if err != nil {
		t.Fatalf("compute base emission: %v", err)
	}

	contextualEmission, err := ComputeEmission(contextual)
	if err != nil {
		t.Fatalf("compute contextual emission: %v", err)
	}

	if contextualEmission.Lday >= baseEmission.Lday {
		t.Fatalf("expected quieter procedure/thrust context to reduce Lday: base=%f contextual=%f", baseEmission.Lday, contextualEmission.Lday)
	}
}

func TestPropagationDecreasesWithDistance(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	cfg := DefaultPropagationConfig()

	near, err := ComputeReceiverPeriodLevels(geo.PointReceiver{ID: "near", Point: geo.Point2D{X: 0, Y: 20}, HeightM: 4}, []AircraftSource{source}, cfg)
	if err != nil {
		t.Fatalf("compute near receiver: %v", err)
	}

	far, err := ComputeReceiverPeriodLevels(geo.PointReceiver{ID: "far", Point: geo.Point2D{X: 0, Y: 400}, HeightM: 4}, []AircraftSource{source}, cfg)
	if err != nil {
		t.Fatalf("compute far receiver: %v", err)
	}

	if near.Lday <= far.Lday {
		t.Fatalf("expected near level > far level: near=%f far=%f", near.Lday, far.Lday)
	}
}

func TestAircraftLdenAggregation(t *testing.T) {
	t.Parallel()

	levels := PeriodLevels{
		Lday:     62,
		Levening: 62,
		Lnight:   62,
	}

	lden := ComputeLden(levels)

	expected := 68.39524300131856
	if math.Abs(lden-expected) > 1e-9 {
		t.Fatalf("unexpected Lden: got %.12f expected %.12f", lden, expected)
	}
}

func TestAttenuationTermsExposePropagationComponents(t *testing.T) {
	t.Parallel()

	source := sampleSource()
	source.LateralOffsetM = 180
	source.BankAngleDeg = 15

	cfg := DefaultPropagationConfig()
	terms := attenuationTerms(100, source, cfg)

	if terms.DistanceM != 100 {
		t.Fatalf("unexpected distance term: %#v", terms)
	}

	if terms.GeometricDB <= 0 || terms.AirDB <= 0 || terms.GroundDB <= 0 {
		t.Fatalf("expected positive attenuation components: %#v", terms)
	}

	if terms.LateralDB <= 0 || terms.OperationDB <= 0 || terms.BankDB <= 0 {
		t.Fatalf("expected positive aircraft adjustment terms: %#v", terms)
	}
}

func TestLateralOffsetRaisesPlanningLevel(t *testing.T) {
	t.Parallel()

	receiver := geo.PointReceiver{ID: "r1", Point: geo.Point2D{X: 0, Y: 50}, HeightM: 4}
	base := sampleSource()
	offset := sampleSource()
	offset.LateralOffsetM = 200

	cfg := DefaultPropagationConfig()

	baseLevels, err := ComputeReceiverPeriodLevels(receiver, []AircraftSource{base}, cfg)
	if err != nil {
		t.Fatalf("compute base receiver levels: %v", err)
	}

	offsetLevels, err := ComputeReceiverPeriodLevels(receiver, []AircraftSource{offset}, cfg)
	if err != nil {
		t.Fatalf("compute offset receiver levels: %v", err)
	}

	if offsetLevels.Lday <= baseLevels.Lday {
		t.Fatalf("expected planning directivity context to increase Lday: base=%f offset=%f", baseLevels.Lday, offsetLevels.Lday)
	}
}

func TestAircraftExportResultBundle(t *testing.T) {
	t.Parallel()

	receivers := []geo.PointReceiver{
		{ID: "r1", Point: geo.Point2D{X: 0, Y: 0}, HeightM: 4},
		{ID: "r2", Point: geo.Point2D{X: 25, Y: 0}, HeightM: 4},
		{ID: "r3", Point: geo.Point2D{X: 0, Y: 25}, HeightM: 4},
		{ID: "r4", Point: geo.Point2D{X: 25, Y: 25}, HeightM: 4},
	}

	outputs, err := ComputeReceiverOutputs(receivers, []AircraftSource{sampleSource()}, DefaultPropagationConfig())
	if err != nil {
		t.Fatalf("compute receiver outputs: %v", err)
	}

	dir := t.TempDir()

	exported, err := ExportResultBundle(dir, outputs, 2, 2)
	if err != nil {
		t.Fatalf("export result bundle: %v", err)
	}

	for _, path := range []string{exported.ReceiverJSONPath, exported.ReceiverCSVPath, exported.RasterMetaPath, exported.RasterDataPath} {
		if _, err := os.Stat(path); err != nil {
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

func TestAircraftGoldenScenario(t *testing.T) {
	t.Parallel()

	var scenario struct {
		Sources    []AircraftSource    `json:"sources"`
		Receivers  []geo.PointReceiver `json:"receivers"`
		GridWidth  int                 `json:"grid_width"`
		GridHeight int                 `json:"grid_height"`
	}

	payload, err := os.ReadFile(testdataPath(t, "aircraft_scenario.json"))
	if err != nil {
		t.Fatalf("read aircraft scenario: %v", err)
	}

	if err := json.Unmarshal(payload, &scenario); err != nil {
		t.Fatalf("decode aircraft scenario: %v", err)
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

	golden.AssertJSONSnapshot(t, testdataPath(t, "aircraft_scenario.golden.json"), snapshot)
}

func TestCnossosAircraftProvenanceMetadataIncludesExpandedKeyParameters(t *testing.T) {
	t.Parallel()

	metadata := ProvenanceMetadata(map[string]string{
		"aircraft_procedure_type": "continuous_descent",
		"aircraft_thrust_mode":    "idle",
		"lateral_offset_m":        "180",
	})

	if metadata["model_version"] != BuiltinModelVersion {
		t.Fatalf("unexpected model_version: %#v", metadata)
	}

	if metadata["compliance_boundary"] != "baseline-preview-expanded-cnossos-aircraft-contract" {
		t.Fatalf("unexpected compliance boundary: %#v", metadata)
	}

	if metadata["key_parameter.aircraft_procedure_type"] != "continuous_descent" || metadata["key_parameter.lateral_offset_m"] != "180" {
		t.Fatalf("expected expanded CNOSSOS aircraft key parameters in metadata: %#v", metadata)
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
		t.Fatal("resolve aircraft test file path")
	}

	base := filepath.Join(filepath.Dir(filePath), "testdata")
	all := append([]string{base}, parts...)

	return filepath.Join(all...)
}

func sampleSource() AircraftSource {
	return AircraftSource{
		ID:         "flight-1",
		SourceType: SourceTypeLine,
		Airport: AirportRef{
			AirportID: "APT",
			RunwayID:  "RWY-09",
		},
		OperationType:         OperationDeparture,
		AircraftClass:         AircraftClassNarrow,
		ProcedureType:         ProcedureStandardSID,
		ThrustMode:            ThrustTakeoff,
		ReferencePowerLevelDB: 108,
		EngineStateFactor:     1.0,
		BankAngleDeg:          0,
		LateralOffsetM:        0,
		FlightTrack: []geo.Point3D{
			{X: -200, Y: 0, Z: 30},
			{X: 0, Y: 0, Z: 120},
			{X: 200, Y: 0, Z: 320},
		},
		MovementDay:     MovementPeriod{MovementsPerHour: 10},
		MovementEvening: MovementPeriod{MovementsPerHour: 5},
		MovementNight:   MovementPeriod{MovementsPerHour: 2},
	}
}
