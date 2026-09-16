package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/io/soundplanimport"
	"github.com/aconiq/backend/internal/report/results"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// referenceGridMapSpacingM is the reference project's RLKDISTANCE. It is the
// same for all four of its grid maps, which is why the spacing never
// discriminates between them and the height has to.
const referenceGridMapSpacingM = 5

// referenceGridMapRun is one grid-map metadata record carrying the two signals
// a selection reads: the geometry the run consumed, and the grid layout its
// `RunCommands` declared.
func referenceGridMapRun(name string, usedBarrier bool, heightM float64) soundplanimport.GridMapMetadata {
	meta := soundplanimport.GridMapMetadata{
		ResultSubFolder: name,
		GMFile:          name + ".GM",
		PointsTotal:     2,
		GeometryFiles:   []string{"geoobjs.geo", "georail.geo"},
		RunLayout:       &soundplanimport.GridMapRunLayout{SpacingM: referenceGridMapSpacingM, HeightM: heightM},
	}

	if usedBarrier {
		meta.GeometryFiles = append(meta.GeometryFiles, soundPlanBarrierGeometryFile)
	}

	return meta
}

// referenceGridMaps mirrors the reference project's four grid maps: the same
// site computed without and with the noise barrier, each at two grid heights.
// Neither signal alone names one run, which is the whole reason the selection
// needs both.
func referenceGridMaps() []soundplanimport.GridMapMetadata {
	return []soundplanimport.GridMapMetadata{
		referenceGridMapRun("RRLK0012", false, 4),
		referenceGridMapRun("RRLK0013", false, 2),
		referenceGridMapRun("RRLK0022", true, 4),
		referenceGridMapRun("RRLK0023", true, 2),
	}
}

// withGridMapFile names the GM payload a synthetic grid-map record points at,
// which referenceGridMapRun otherwise derives from the run name.
func withGridMapFile(meta soundplanimport.GridMapMetadata, gmFile string) soundplanimport.GridMapMetadata {
	meta.GMFile = gmFile

	return meta
}

// TestSelectSoundPlanGridMapRunNeedsBothSignals is the defect this selection
// exists to close: the raster path compared one Aconiq run against all four
// grid maps, so at least two of the four measured a scenario the model does not
// describe.
func TestSelectSoundPlanGridMapRunNeedsBothSignals(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		modelHasBarriers bool
		gridMapHeightM   float64
		wantDir          string
	}{
		{"barrier at 2 m", true, 2, "RRLK0023"},
		{"barrier at 4 m", true, 4, "RRLK0022"},
		{"no barrier at 2 m", false, 2, "RRLK0013"},
		{"no barrier at 4 m", false, 4, "RRLK0012"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			selection, err := selectSoundPlanGridMapRun(referenceGridMaps(), "", testCase.modelHasBarriers, testCase.gridMapHeightM)
			if err != nil {
				t.Fatalf("selectSoundPlanGridMapRun: %v", err)
			}

			if selection.Dir != testCase.wantDir {
				t.Fatalf("dir = %q, want %q", selection.Dir, testCase.wantDir)
			}

			if selection.Selection != gridRunSelectionGridHeight {
				t.Fatalf("selection = %q, want %q", selection.Selection, gridRunSelectionGridHeight)
			}

			if len(selection.Warnings) != 0 {
				t.Fatalf("warnings = %v, want none when both signals agree on one run", selection.Warnings)
			}

			// Every grid map stays a candidate on the record, so the report can
			// still say what the choice was made out of.
			if len(selection.Candidates) != 4 {
				t.Fatalf("candidates = %v, want all four grid maps", selection.Candidates)
			}
		})
	}
}

// TestSelectSoundPlanGridMapRunBarrierAloneLeavesTwo pins the finding that made
// the second signal necessary: reusing the receiver path's barrier test alone
// narrows four runs to two, not to one.
func TestSelectSoundPlanGridMapRunBarrierAloneLeavesTwo(t *testing.T) {
	t.Parallel()

	// The project recorded no RLKHEIGHT, so there is nothing to test the grid
	// height against and the barrier signal is all there is.
	selection, err := selectSoundPlanGridMapRun(referenceGridMaps(), "", true, 0)
	if err != nil {
		t.Fatalf("selectSoundPlanGridMapRun: %v", err)
	}

	if selection.Selection != resultRunSelectionAmbiguous {
		t.Fatalf("selection = %q, want %q", selection.Selection, resultRunSelectionAmbiguous)
	}

	if selection.Dir != "RRLK0023" {
		t.Fatalf("dir = %q, want the last of the two barrier runs by name", selection.Dir)
	}

	if len(selection.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one saying the choice was arbitrary", selection.Warnings)
	}

	// The warning has to name the pool it could not tell apart and the flag
	// that settles it; naming all four would overstate what was undecided.
	warning := selection.Warnings[0]
	for _, want := range []string{"RRLK0022", "RRLK0023", "--soundplan-grid-run"} {
		if !strings.Contains(warning, want) {
			t.Fatalf("warning %q does not mention %q", warning, want)
		}
	}

	if strings.Contains(warning, "RRLK0012") {
		t.Fatalf("warning %q names a run the barrier signal already ruled out", warning)
	}
}

// TestSelectSoundPlanGridMapRunWithoutEvidence covers the import report that
// carries no geometry list and no declared layout — every report written before
// those fields existed, and every synthetic fixture in compare_raster_test.go.
// Absent evidence must fall back, not fail.
func TestSelectSoundPlanGridMapRunWithoutEvidence(t *testing.T) {
	t.Parallel()

	blank := []soundplanimport.GridMapMetadata{
		{ResultSubFolder: "RS01", GMFile: "RRLK0010.GM"},
		{ResultSubFolder: "RS02", GMFile: "RRLK0020.GM"},
	}

	selection, err := selectSoundPlanGridMapRun(blank, "", true, 2)
	if err != nil {
		t.Fatalf("selectSoundPlanGridMapRun: %v", err)
	}

	if selection.Dir != "RS02" || selection.Selection != resultRunSelectionAmbiguous {
		t.Fatalf("dir = %q (%s), want RS02 (%s)", selection.Dir, selection.Selection, resultRunSelectionAmbiguous)
	}

	// One grid map is not a choice, so no evidence is needed to make it.
	only, err := selectSoundPlanGridMapRun(blank[:1], "", true, 2)
	if err != nil {
		t.Fatalf("selectSoundPlanGridMapRun: %v", err)
	}

	if only.Dir != "RS01" || only.Selection != resultRunSelectionOnly {
		t.Fatalf("dir = %q (%s), want RS01 (%s)", only.Dir, only.Selection, resultRunSelectionOnly)
	}

	// A grid map that names no result subfolder is not a candidate at all, and
	// the caller has to be able to see that nothing was selected.
	none, err := selectSoundPlanGridMapRun([]soundplanimport.GridMapMetadata{{GMFile: "RRLK0010.GM"}}, "", true, 2)
	if err != nil {
		t.Fatalf("selectSoundPlanGridMapRun: %v", err)
	}

	if none.Dir != "" {
		t.Fatalf("dir = %q, want none", none.Dir)
	}
}

func TestSelectSoundPlanGridMapRunExplicit(t *testing.T) {
	t.Parallel()

	// An explicit run wins over both signals, including over the run they would
	// otherwise have chosen.
	selection, err := selectSoundPlanGridMapRun(referenceGridMaps(), "RRLK0012", true, 2)
	if err != nil {
		t.Fatalf("selectSoundPlanGridMapRun: %v", err)
	}

	if selection.Dir != "RRLK0012" || selection.Selection != resultRunSelectionExplicit {
		t.Fatalf("dir = %q (%s), want RRLK0012 (%s)", selection.Dir, selection.Selection, resultRunSelectionExplicit)
	}

	if _, err := selectSoundPlanGridMapRun(referenceGridMaps(), "RRLK9999", true, 2); err == nil {
		t.Fatal("expected an error for a grid-map run that does not exist")
	}
}

// TestPrepareSoundPlanRasterCompareDecodesOnlyTheSelectedRun carries the
// selection through the preparation: the run that gets decoded, and whose grid
// layout therefore places the receivers, is the selected one — not whichever
// grid map the import happened to list first.
func TestPrepareSoundPlanRasterCompareDecodesOnlyTheSelectedRun(t *testing.T) {
	t.Parallel()

	barrier := map[string]any{
		"type":       "Feature",
		"properties": map[string]any{"id": "wall", "kind": "barrier", "height_m": 3.0},
		"geometry":   map[string]any{"type": "LineString", "coordinates": []any{[]any{0, 1}, []any{5, 1}}},
	}

	projectRoot, modelPath := rasterCompareProject(t, barrier)

	// The second grid map carries a different payload, so a comparison against
	// the wrong run is visible in the decoded values rather than only in the
	// reported name.
	writeRasterCompareGridMap(t, projectRoot, "RS02", "RRLK0020.GM",
		[]testGridCell{{ground: -1, day: 0, night: 0, flag: 1}, {ground: 110, day: 70, night: 60, flag: 1}})

	report := soundPlanImportReport{
		SourcePath:      "soundplan",
		ProjectCRS:      "EPSG:25832",
		GridResolutionM: 5,
		GridMapHeightM:  4,
		CalcArea:        &soundPlanImportCalcArea{Points: []soundPlanPoint{{X: 0, Y: 0}, {X: 5, Y: 0}, {X: 5, Y: 5}, {X: 0, Y: 5}, {X: 0, Y: 0}}},
		GridMaps: []soundplanimport.GridMapMetadata{
			withGridMapFile(referenceGridMapRun("RS01", false, 4), "RRLK0010.GM"),
			withGridMapFile(referenceGridMapRun("RS02", true, 4), "RRLK0020.GM"),
		},
	}

	prep, hasPrep, err := prepareSoundPlanRasterCompare(projectRoot, report, modelPath, "")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if !hasPrep {
		t.Fatal("expected a raster comparison preparation")
	}

	t.Cleanup(func() {
		_ = os.Remove(filepath.Join(projectRoot, filepath.FromSlash(defaultRasterCompareArtifactPath)))

		cleanupRasterComparePreparation(prep)
	})

	if got, want := prep.report.SoundPlanRasterRun, "RS02"; got != want {
		t.Fatalf("soundplan_raster_run = %q, want %q — the model carries a barrier", got, want)
	}

	if got, want := prep.report.SoundPlanRasterRunSelection, resultRunSelectionGeometry; got != want {
		t.Fatalf("selection = %q, want %q", got, want)
	}

	if len(prep.decodedRuns) != 1 {
		t.Fatalf("decoded runs = %d, want exactly the selected one", len(prep.decodedRuns))
	}

	if got := prep.decodedRuns[0].metadata.ResultSubFolder; got != "RS02" {
		t.Fatalf("decoded run = %q, want RS02", got)
	}

	// Both grid maps stay on the record; one of them was compared.
	if len(prep.report.SoundPlanRuns) != 2 || len(prep.report.SoundPlanRasterRunCandidates) != 2 {
		t.Fatalf("discovered runs = %d, candidates = %v, want both grid maps in each",
			len(prep.report.SoundPlanRuns), prep.report.SoundPlanRasterRunCandidates)
	}

	table := results.ReceiverTable{
		IndicatorOrder: []string{schall03.IndicatorLrDay, schall03.IndicatorLrNight},
		Unit:           "dB(A)",
		Records: []results.ReceiverRecord{{
			ID:      prep.syntheticReceiverIDs[0],
			HeightM: 4,
			Values:  map[string]float64{schall03.IndicatorLrDay: 70, schall03.IndicatorLrNight: 60},
		}},
	}

	reportOut, artifact, err := finalizeSoundPlanRasterCompare(projectRoot, prep, table, 0.5)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}

	if len(reportOut.Runs) != 1 || len(artifact.Runs) != 1 {
		t.Fatalf("summary runs = %d, artifact runs = %d, want one comparison each", len(reportOut.Runs), len(artifact.Runs))
	}

	if got := artifact.Runs[0].ResultSubFolder; got != "RS02" {
		t.Fatalf("artifact run = %q, want RS02", got)
	}

	// RS02's day cell holds 70 dB and RS01's holds 50, so a zero delta is the
	// proof that the values compared came from the selected run.
	if got := artifact.Runs[0].Records[0].DeltaDayDB; got != 0 {
		t.Fatalf("delta = %v dB, want 0 — the comparison read the unselected grid map's values", got)
	}

	if got := artifact.SoundPlanRasterRun; got != "RS02" {
		t.Fatalf("artifact soundplan_raster_run = %q, want RS02", got)
	}
}

// TestPrepareSoundPlanRasterCompareRejectsUnknownGridRun keeps an explicit flag
// that names nothing a hard error rather than a silent fallback, exactly as
// --soundplan-run is.
func TestPrepareSoundPlanRasterCompareRejectsUnknownGridRun(t *testing.T) {
	t.Parallel()

	projectRoot, modelPath := rasterCompareProject(t)

	report := soundPlanImportReport{
		SourcePath:      "soundplan",
		ProjectCRS:      "EPSG:25832",
		GridResolutionM: 5,
		GridMaps:        []soundplanimport.GridMapMetadata{{ResultSubFolder: "RS01", GMFile: "RRLK0010.GM", PointsTotal: 2}},
	}

	if _, _, err := prepareSoundPlanRasterCompare(projectRoot, report, modelPath, "RS99"); err == nil {
		t.Fatal("expected an error for a grid-map run that does not exist")
	}
}
