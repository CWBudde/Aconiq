package acoustics

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/report/results"
)

func TestComputeLdenAppliesTheDirectiveWeighting(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		levels PeriodLevels
		want   float64
	}{
		{
			// The weights are 12/4/8 hours and the penalties +0/+5/+10 dB, so
			// levels of 55/50/45 put the same energy in all three periods and
			// Lden comes back as exactly the day level. A change to any weight
			// or penalty breaks this identity.
			name:   "penalties cancel the period split",
			levels: PeriodLevels{Lday: 55, Levening: 50, Lnight: 45},
			want:   55,
		},
		{
			name:   "equal levels gain the weighted penalty",
			levels: PeriodLevels{Lday: 60, Levening: 60, Lnight: 60},
			want:   66.39524300131856,
		},
		{
			name:   "the penalty is independent of the level",
			levels: PeriodLevels{Lday: 0, Levening: 0, Lnight: 0},
			want:   6.39524300131856,
		},
		{
			name:   "night dominates when day and evening are silent",
			levels: PeriodLevels{Lday: math.Inf(-1), Levening: math.Inf(-1), Lnight: 60},
			want:   65.22878745280337,
		},
		{
			name:   "no energy at all reports the silence sentinel",
			levels: PeriodLevels{Lday: math.Inf(-1), Levening: math.Inf(-1), Lnight: math.Inf(-1)},
			want:   SilenceDB,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := ComputeLden(testCase.levels)
			if math.Abs(got-testCase.want) > 1e-9 {
				t.Fatalf("unexpected Lden: got %v want %v", got, testCase.want)
			}
		})
	}
}

func TestToReceiverIndicatorsCarriesThePeriodsUnchanged(t *testing.T) {
	t.Parallel()

	levels := PeriodLevels{Lday: 61, Levening: 57, Lnight: 52}

	indicators := levels.ToReceiverIndicators()

	if indicators.Lday != levels.Lday || indicators.Levening != levels.Levening || indicators.Lnight != levels.Lnight {
		t.Fatalf("periods must pass through unchanged: %+v from %+v", indicators, levels)
	}

	if indicators.Lden != ComputeLden(levels) {
		t.Fatalf("Lden must be the computed one: got %v", indicators.Lden)
	}
}

// The receiver table keys and the order they are written in are a published
// contract: a consumer reading two modules' bundles compares them column by
// column, so the two must not drift apart.
func TestValuesCoverExactlyTheIndicatorOrder(t *testing.T) {
	t.Parallel()

	values := ReceiverIndicators{Lday: 1, Levening: 2, Lnight: 3, Lden: 4}.Values()

	order := IndicatorOrder()
	if len(values) != len(order) {
		t.Fatalf("expected %d values, got %d", len(order), len(values))
	}

	want := map[string]float64{IndicatorLden: 4, IndicatorLnight: 3, IndicatorLday: 1, IndicatorLevening: 2}

	for _, name := range order {
		got, ok := values[name]
		if !ok {
			t.Fatalf("indicator %s missing from the value map", name)
		}

		if got != want[name] {
			t.Fatalf("indicator %s: got %v want %v", name, got, want[name])
		}
	}
}

func TestExportENDBundleWritesTheSharedLayout(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	exported, err := ExportENDBundle(dir, "some-standard", sampleOutputs(), results.GridLayout{Width: 2, Height: 2})
	if err != nil {
		t.Fatalf("export bundle: %v", err)
	}

	for _, path := range []string{exported.ReceiverJSONPath, exported.ReceiverCSVPath, exported.RasterMetaPath, exported.RasterDataPath} {
		_, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("expected exported file %s: %v", path, statErr)
		}
	}

	// The raster is named after the standard that produced it: this is the one
	// thing that differs between modules sharing this layout, and it is what
	// keeps two bundles in one directory from overwriting each other.
	if filepath.Base(exported.RasterMetaPath) != "some-standard.json" {
		t.Fatalf("unexpected raster metadata name: %s", exported.RasterMetaPath)
	}

	table, err := results.LoadReceiverTableJSON(exported.ReceiverJSONPath)
	if err != nil {
		t.Fatalf("load receiver table: %v", err)
	}

	if strings.Join(table.IndicatorOrder, ",") != strings.Join(IndicatorOrder(), ",") {
		t.Fatalf("unexpected indicator order: %v", table.IndicatorOrder)
	}

	if len(table.Records) != 4 {
		t.Fatalf("expected 4 receiver records, got %d", len(table.Records))
	}

	raster, err := results.LoadRaster(exported.RasterMetaPath)
	if err != nil {
		t.Fatalf("load raster: %v", err)
	}

	meta := raster.Metadata()
	if meta.Bands != 2 || strings.Join(meta.BandNames, ",") != IndicatorLden+","+IndicatorLnight {
		t.Fatalf("unexpected raster bands: %d %v", meta.Bands, meta.BandNames)
	}
}

func TestExportENDBundleRejectsInputItCannotWrite(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		baseDir    string
		standardID string
		outputs    []ReceiverOutput
		gridWidth  int
		gridHeight int
	}{
		{name: "no base dir", standardID: "s", outputs: sampleOutputs(), gridWidth: 2, gridHeight: 2},
		{name: "no standard id", baseDir: "dir", outputs: sampleOutputs(), gridWidth: 2, gridHeight: 2},
		{name: "no outputs", baseDir: "dir", standardID: "s", gridWidth: 2, gridHeight: 2},
		{name: "zero grid", baseDir: "dir", standardID: "s", outputs: sampleOutputs()},
		{name: "grid does not match the outputs", baseDir: "dir", standardID: "s", outputs: sampleOutputs(), gridWidth: 3, gridHeight: 2},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			baseDir := testCase.baseDir
			if baseDir != "" {
				baseDir = t.TempDir()
			}

			_, err := ExportENDBundle(baseDir, testCase.standardID, testCase.outputs, results.GridLayout{Width: testCase.gridWidth, Height: testCase.gridHeight})
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func sampleOutputs() []ReceiverOutput {
	outputs := make([]ReceiverOutput, 0, 4)
	for index := range 4 {
		outputs = append(outputs, ReceiverOutput{
			Receiver: geo.PointReceiver{
				ID:      "r" + string(rune('1'+index)),
				Point:   geo.Point2D{X: float64(index % 2), Y: float64(index / 2)},
				HeightM: 4,
			},
			Indicators: PeriodLevels{Lday: 60 + float64(index), Levening: 55, Lnight: 50}.ToReceiverIndicators(),
		})
	}

	return outputs
}
