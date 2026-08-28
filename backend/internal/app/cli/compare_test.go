package cli

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/aconiq/backend/internal/domain/project"
	"github.com/aconiq/backend/internal/standards/schall03"
)

func TestCompareSoundPlanReceivers(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	soundPlanDir := soundPlanInteropPath(t)

	mustRunCLI(t, "--project", projectDir, "init", "--name", "CompareSoundPLAN", "--crs", "EPSG:25832")
	mustRunCLI(t, "--project", projectDir, "import", "--from-soundplan", soundPlanDir)
	mustRunCLI(t, "--project", projectDir, "compare")

	reportPath := filepath.Join(projectDir, ".noise", "artifacts", "soundplan-receiver-compare.json")

	payload, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read compare report: %v", err)
	}

	var report soundPlanCompareReport
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("decode compare report: %v", err)
	}

	if report.Command != "compare" {
		t.Fatalf("command = %q, want compare", report.Command)
	}

	if report.RunID == "" {
		t.Fatal("expected run id in compare report")
	}

	if report.MatchedReceiverCount == 0 {
		t.Fatal("expected at least one matched receiver")
	}

	if report.MatchedReceiverCount+report.UnmatchedAconiqCount != 77 {
		t.Fatalf("matched + unmatched Aconiq receivers = %d, want 77", report.MatchedReceiverCount+report.UnmatchedAconiqCount)
	}

	if report.Stats[schall03.IndicatorLrDay].Count != report.MatchedReceiverCount {
		t.Fatalf("day stats count = %d, want %d", report.Stats[schall03.IndicatorLrDay].Count, report.MatchedReceiverCount)
	}

	assertCompareRecordsAreSelfConsistent(t, report)
	assertCompareDeltasWithinObservedBand(t, report)

	if report.Raster == nil {
		t.Fatal("expected raster metadata section in compare report")
	}

	if report.Raster.Status != "heuristic_scanline_compare" {
		t.Fatalf("raster status = %q, want heuristic_scanline_compare", report.Raster.Status)
	}

	if len(report.Raster.SoundPlanRuns) != 4 {
		t.Fatalf("raster run count = %d, want 4", len(report.Raster.SoundPlanRuns))
	}

	if report.Raster.ArtifactPath == "" {
		t.Fatal("expected raster artifact path in compare report")
	}

	if len(report.Raster.Runs) != 4 {
		t.Fatalf("raster summary run count = %d, want 4", len(report.Raster.Runs))
	}

	for _, run := range report.Raster.Runs {
		if run.ComparedCellCount == 0 {
			t.Fatalf("%s compared_cell_count = 0, want > 0", run.ResultSubFolder)
		}
	}

	rasterPayload, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(report.Raster.ArtifactPath)))
	if err != nil {
		t.Fatalf("read raster compare artifact: %v", err)
	}

	var rasterArtifact soundPlanRasterCompareArtifact
	if err := json.Unmarshal(rasterPayload, &rasterArtifact); err != nil {
		t.Fatalf("decode raster compare artifact: %v", err)
	}

	if rasterArtifact.Status != "heuristic_scanline_compare" {
		t.Fatalf("raster artifact status = %q, want heuristic_scanline_compare", rasterArtifact.Status)
	}

	if len(rasterArtifact.Runs) != 4 {
		t.Fatalf("raster artifact run count = %d, want 4", len(rasterArtifact.Runs))
	}

	manifestPayload, err := os.ReadFile(filepath.Join(projectDir, ".noise", "project.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var proj project.Project
	if err := json.Unmarshal(manifestPayload, &proj); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}

	if _, ok := latestRun(proj.Runs); !ok {
		t.Fatal("expected compare to create a run")
	}
}

// Recorded SoundPLAN agreement bounds.
//
// These record what `aconiq compare` actually produces against the reference
// SoundPLAN project, measured on 2026-08-28 with the fixture present locally:
//
//	LrDay    mean_abs 24.052  p95_abs 37.859  max_abs 39.745  exceedances 54/54
//	LrNight  mean_abs 22.630  p95_abs 36.098  max_abs 37.965  exceedances 54/54
//
// The previous measurement was LrDay 25.110 / LrNight 23.329. Removing the
// spurious 10 lg(n + 1) flow shift from schall03/emission.go (PLAN.md 1.2)
// accounts for the whole of the improvement; the shift sat on the data-pack
// path, which is the path the CLI runs.
//
// Aconiq reads systematically ~25 dB HIGH against SoundPLAN over all 54
// matched receivers, and every single receiver exceeds the 0.5 dB tolerance.
// The bounds below are therefore NOT a conformance tolerance and must never be
// read as one: they are a regression guard that pins the current, bad state so
// it cannot silently get worse, with roughly 20 % headroom on the observed
// values.
//
// They must be tightened - by a large factor - once the schall03 pipeline and
// the receiver matching actually agree with the reference (PLAN.md Priority 3
// and Priority 4). See also assertCompareRecordsAreSelfConsistent, which does
// hold the compare command itself to an exact standard.
const (
	maxObservedMeanAbsDeltaDB = 30.0
	maxObservedMaxAbsDeltaDB  = 45.0
)

// assertCompareRecordsAreSelfConsistent checks the parts of the compare output
// that must be exact regardless of how well the model agrees with SoundPLAN:
// every delta is the difference of the two levels it is derived from, and
// every aggregate in `stats` is the aggregate of the records it summarizes.
func assertCompareRecordsAreSelfConsistent(t *testing.T, report soundPlanCompareReport) {
	t.Helper()

	if len(report.Records) != report.MatchedReceiverCount {
		t.Fatalf("record count = %d, want matched_receiver_count %d", len(report.Records), report.MatchedReceiverCount)
	}

	dayAbs := make([]float64, 0, len(report.Records))
	nightAbs := make([]float64, 0, len(report.Records))

	for _, record := range report.Records {
		if record.AconiqID == "" {
			t.Fatalf("record without an Aconiq receiver ID: %#v", record)
		}

		if record.MatchStrategy != "coordinates" && record.MatchStrategy != "ordinal" {
			t.Fatalf("unknown match strategy %q in %#v", record.MatchStrategy, record)
		}

		for name, value := range map[string]float64{
			"aconiq_lr_day":   record.AconiqLrDay,
			"soundplan_zb1":   record.SoundPlanZB1,
			"delta_day_db":    record.DeltaDayDB,
			"aconiq_lr_night": record.AconiqLrNight,
			"soundplan_zb2":   record.SoundPlanZB2,
			"delta_night_db":  record.DeltaNightDB,
		} {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				t.Fatalf("%s of %s is not finite: %v", name, record.AconiqID, value)
			}
		}

		if want := record.AconiqLrDay - record.SoundPlanZB1; math.Abs(record.DeltaDayDB-want) > 1e-9 {
			t.Fatalf("%s delta_day_db = %v, want %v", record.AconiqID, record.DeltaDayDB, want)
		}

		if want := record.AconiqLrNight - record.SoundPlanZB2; math.Abs(record.DeltaNightDB-want) > 1e-9 {
			t.Fatalf("%s delta_night_db = %v, want %v", record.AconiqID, record.DeltaNightDB, want)
		}

		dayAbs = append(dayAbs, math.Abs(record.DeltaDayDB))
		nightAbs = append(nightAbs, math.Abs(record.DeltaNightDB))
	}

	assertIndicatorStatsMatchRecords(t, schall03.IndicatorLrDay, report.Stats[schall03.IndicatorLrDay], dayAbs, report.ToleranceDB)
	assertIndicatorStatsMatchRecords(t, schall03.IndicatorLrNight, report.Stats[schall03.IndicatorLrNight], nightAbs, report.ToleranceDB)
}

// assertIndicatorStatsMatchRecords recomputes the reported aggregates from the
// per-receiver records. The aggregates are computed by the compare command
// from a separate accumulator, so this cross-checks the two code paths against
// each other rather than a function against itself.
func assertIndicatorStatsMatchRecords(t *testing.T, indicator string, stats compareIndicatorStats, absDeltas []float64, toleranceDB float64) {
	t.Helper()

	if stats.Count != len(absDeltas) {
		t.Fatalf("%s count = %d, want %d", indicator, stats.Count, len(absDeltas))
	}

	if len(absDeltas) == 0 {
		t.Fatalf("%s has no matched receivers to compare", indicator)
	}

	wantMax := absDeltas[0]
	wantSum := 0.0
	wantExceeding := 0

	for _, value := range absDeltas {
		wantMax = math.Max(wantMax, value)
		wantSum += value

		if value > toleranceDB {
			wantExceeding++
		}
	}

	wantMean := wantSum / float64(len(absDeltas))

	if math.Abs(stats.MaxAbsDeltaDB-wantMax) > 1e-9 {
		t.Fatalf("%s max_abs_delta_db = %v, want %v", indicator, stats.MaxAbsDeltaDB, wantMax)
	}

	if math.Abs(stats.MeanAbsDeltaDB-wantMean) > 1e-9 {
		t.Fatalf("%s mean_abs_delta_db = %v, want %v", indicator, stats.MeanAbsDeltaDB, wantMean)
	}

	if stats.ToleranceExceeding != wantExceeding {
		t.Fatalf("%s tolerance_exceeding = %d, want %d at tolerance %v dB", indicator, stats.ToleranceExceeding, wantExceeding, toleranceDB)
	}

	// The 95th percentile must be an actually observed value and must sit
	// between the mean and the maximum.
	if !slices.ContainsFunc(absDeltas, func(value float64) bool { return math.Abs(value-stats.P95AbsDeltaDB) <= 1e-9 }) {
		t.Fatalf("%s p95_abs_delta_db = %v is not one of the observed deltas", indicator, stats.P95AbsDeltaDB)
	}

	if stats.P95AbsDeltaDB < stats.MeanAbsDeltaDB-1e-9 || stats.P95AbsDeltaDB > stats.MaxAbsDeltaDB+1e-9 {
		t.Fatalf("%s p95 %v is not between mean %v and max %v", indicator, stats.P95AbsDeltaDB, stats.MeanAbsDeltaDB, stats.MaxAbsDeltaDB)
	}
}

// assertCompareDeltasWithinObservedBand is the regression guard described on
// maxObservedMeanAbsDeltaDB. It is deliberately loose; read that comment before
// treating either bound as a tolerance.
func assertCompareDeltasWithinObservedBand(t *testing.T, report soundPlanCompareReport) {
	t.Helper()

	for _, indicator := range []string{schall03.IndicatorLrDay, schall03.IndicatorLrNight} {
		stats := report.Stats[indicator]

		if stats.MeanAbsDeltaDB > maxObservedMeanAbsDeltaDB {
			t.Fatalf(
				"%s mean_abs_delta_db = %.3f exceeds the recorded regression bound %.1f dB; agreement with SoundPLAN got worse",
				indicator, stats.MeanAbsDeltaDB, maxObservedMeanAbsDeltaDB,
			)
		}

		if stats.MaxAbsDeltaDB > maxObservedMaxAbsDeltaDB {
			t.Fatalf(
				"%s max_abs_delta_db = %.3f exceeds the recorded regression bound %.1f dB; agreement with SoundPLAN got worse",
				indicator, stats.MaxAbsDeltaDB, maxObservedMaxAbsDeltaDB,
			)
		}

		if stats.ToleranceExceeding > stats.Count {
			t.Fatalf("%s tolerance_exceeding = %d exceeds count %d", indicator, stats.ToleranceExceeding, stats.Count)
		}
	}
}
