package cli

import (
	"encoding/json"
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

// TestSelectSoundPlanGridMapRunContradictedGeometry covers the case the
// geometry filter used to hand straight to the height filter: every candidate
// declared its geometry, and every one of them disagrees with the model. The
// model provably describes none of these runs, so a unique height match is not
// a discriminator — it is one wrong answer out of a set of wrong answers, and
// reporting it as grid_height_match would present it as the confident one.
func TestSelectSoundPlanGridMapRunContradictedGeometry(t *testing.T) {
	t.Parallel()

	// An edited --model added a barrier the imported bundle never computed
	// with: both grid maps used GeoWand.geo, the model does not.
	barrierOnly := []soundplanimport.GridMapMetadata{
		referenceGridMapRun("RRLK0022", true, 4),
		referenceGridMapRun("RRLK0023", true, 2),
	}

	// RLKHEIGHT 2 m singles RRLK0023 out on height alone, which is exactly the
	// confident-looking answer this must not give.
	selection, err := selectSoundPlanGridMapRun(barrierOnly, "", false, 2)
	if err != nil {
		t.Fatalf("selectSoundPlanGridMapRun: %v", err)
	}

	if selection.Selection == gridRunSelectionGridHeight {
		t.Fatal("a height match was reported for a pool whose geometry rules every candidate out")
	}

	if selection.Selection != gridRunSelectionContradicted {
		t.Fatalf("selection = %q, want %q", selection.Selection, gridRunSelectionContradicted)
	}

	// Still a selection: the command has to stay usable for "I know, compare
	// anyway", so refusing outright would be the wrong trade.
	if selection.Dir != "RRLK0023" {
		t.Fatalf("dir = %q, want the last candidate by name order", selection.Dir)
	}

	if len(selection.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one naming the contradiction", selection.Warnings)
	}

	warning := selection.Warnings[0]
	for _, want := range []string{"RRLK0022", "RRLK0023", "--soundplan-grid-run", "noise barrier"} {
		if !strings.Contains(warning, want) {
			t.Fatalf("warning %q does not mention %q", warning, want)
		}
	}

	// The mirror case: the model carries the barrier and no run did.
	mirror, err := selectSoundPlanGridMapRun(
		[]soundplanimport.GridMapMetadata{
			referenceGridMapRun("RRLK0012", false, 4),
			referenceGridMapRun("RRLK0013", false, 2),
		}, "", true, 2,
	)
	if err != nil {
		t.Fatalf("selectSoundPlanGridMapRun: %v", err)
	}

	if mirror.Selection != gridRunSelectionContradicted {
		t.Fatalf("selection = %q, want %q", mirror.Selection, gridRunSelectionContradicted)
	}

	// An explicit run is still the user's call, contradiction or not.
	explicit, err := selectSoundPlanGridMapRun(barrierOnly, "RRLK0022", false, 2)
	if err != nil {
		t.Fatalf("selectSoundPlanGridMapRun: %v", err)
	}

	if explicit.Dir != "RRLK0022" || explicit.Selection != resultRunSelectionExplicit {
		t.Fatalf("dir = %q (%s), want RRLK0022 (%s)", explicit.Dir, explicit.Selection, resultRunSelectionExplicit)
	}
}

// TestSelectSoundPlanGridMapRunHeightDecidesWithoutGeometry is the other half
// of the distinction: absent geometry evidence is not contradicted evidence,
// and the height must still settle the choice on its own. This is every import
// report written before GridMapMetadata.GeometryFiles existed, so it has to
// keep behaving exactly as it did.
func TestSelectSoundPlanGridMapRunHeightDecidesWithoutGeometry(t *testing.T) {
	t.Parallel()

	noGeometry := []soundplanimport.GridMapMetadata{
		{ResultSubFolder: "RS01", GMFile: "RRLK0010.GM", RunLayout: &soundplanimport.GridMapRunLayout{SpacingM: referenceGridMapSpacingM, HeightM: 4}},
		{ResultSubFolder: "RS02", GMFile: "RRLK0020.GM", RunLayout: &soundplanimport.GridMapRunLayout{SpacingM: referenceGridMapSpacingM, HeightM: 2}},
	}

	selection, err := selectSoundPlanGridMapRun(noGeometry, "", true, 2)
	if err != nil {
		t.Fatalf("selectSoundPlanGridMapRun: %v", err)
	}

	if selection.Dir != "RS02" || selection.Selection != gridRunSelectionGridHeight {
		t.Fatalf("dir = %q (%s), want RS02 (%s)", selection.Dir, selection.Selection, gridRunSelectionGridHeight)
	}

	if len(selection.Warnings) != 0 {
		t.Fatalf("warnings = %v, want none when the height names exactly one run", selection.Warnings)
	}

	// A partial list is absent evidence too. One candidate disagrees and the
	// other never said, so the pool has not been ruled out — the unknown run
	// could still be the right one, and the height is the only signal left.
	partial := []soundplanimport.GridMapMetadata{
		referenceGridMapRun("RS01", true, 4),
		{ResultSubFolder: "RS02", GMFile: "RRLK0020.GM", RunLayout: &soundplanimport.GridMapRunLayout{SpacingM: referenceGridMapSpacingM, HeightM: 2}},
	}

	mixed, err := selectSoundPlanGridMapRun(partial, "", false, 2)
	if err != nil {
		t.Fatalf("selectSoundPlanGridMapRun: %v", err)
	}

	if mixed.Dir != "RS02" || mixed.Selection != gridRunSelectionGridHeight {
		t.Fatalf("dir = %q (%s), want RS02 (%s)", mixed.Dir, mixed.Selection, gridRunSelectionGridHeight)
	}
}

// TestPrepareSoundPlanRasterCompareUsesSelectedRunHeight pins where the
// synthetic receivers are placed once the comparison has chosen a run.
//
// --soundplan-grid-run names a run outright, and that run need not have been
// computed at the project's current RLKHEIGHT. Placing the Aconiq receivers at
// RLKHEIGHT anyway would compare levels at one height against SoundPLAN cells
// at another while the report states the run was chosen deliberately — the
// mismatch selecting a single run exists to remove.
func TestPrepareSoundPlanRasterCompareUsesSelectedRunHeight(t *testing.T) {
	t.Parallel()

	projectRoot, modelPath := rasterCompareProject(t)

	writeRasterCompareGridMap(t, projectRoot, "RS02", "RRLK0020.GM",
		[]testGridCell{{ground: -1, day: 0, night: 0, flag: 1}, {ground: 110, day: 70, night: 60, flag: 1}})

	// The project's RLKHEIGHT is 4 m and RS02 was computed at 2 m. Neither run
	// carries the barrier and the model has none, so nothing but the explicit
	// flag picks RS02 — and the height then has to follow the flag.
	report := soundPlanImportReport{
		SourcePath:      "soundplan",
		ProjectCRS:      "EPSG:25832",
		GridResolutionM: 5,
		GridMapHeightM:  4,
		CalcArea:        &soundPlanImportCalcArea{Points: []soundPlanPoint{{X: 0, Y: 0}, {X: 5, Y: 0}, {X: 5, Y: 5}, {X: 0, Y: 5}, {X: 0, Y: 0}}},
		GridMaps: []soundplanimport.GridMapMetadata{
			withGridMapFile(referenceGridMapRun("RS01", false, 4), "RRLK0010.GM"),
			withGridMapFile(referenceGridMapRun("RS02", false, 2), "RRLK0020.GM"),
		},
	}

	prep, hasPrep, err := prepareSoundPlanRasterCompare(projectRoot, report, modelPath, "RS02")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if !hasPrep {
		t.Fatal("expected a raster comparison preparation")
	}

	t.Cleanup(func() { cleanupRasterComparePreparation(prep) })

	if got, want := prep.report.SoundPlanRasterRun, "RS02"; got != want {
		t.Fatalf("soundplan_raster_run = %q, want %q", got, want)
	}

	if got := prep.report.ReceiverHeightM; got != 2 {
		t.Fatalf("receiver_height_m = %v, want 2 — the selected run's height, not RLKHEIGHT", got)
	}

	// What the report states has to be what the receivers were built at, so the
	// heights are read back out of the model the run will actually consume.
	for id, heightM := range syntheticRasterReceiverHeights(t, prep) {
		if heightM != 2 {
			t.Fatalf("synthetic receiver %q sits at %v m, want 2 m", id, heightM)
		}
	}

	// A disagreement with the project's own setting is the user's call, but it
	// is not allowed to be silent: the warning names both numbers.
	var found string

	for _, warning := range prep.report.Warnings {
		if strings.Contains(warning, "RLKHEIGHT") {
			found = warning
		}
	}

	if found == "" {
		t.Fatalf("no warning names the height disagreement; warnings = %v", prep.report.Warnings)
	}

	for _, want := range []string{"RS02", "2 m", "4 m"} {
		if !strings.Contains(found, want) {
			t.Fatalf("warning %q does not mention %q", found, want)
		}
	}
}

// TestSelectedRunReceiverHeightFallsBackToProject covers the runs that declared
// no usable layout, which is every import report written before
// GridMapMetadata.RunLayout existed.
func TestSelectedRunReceiverHeightFallsBackToProject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		meta        soundplanimport.GridMapMetadata
		imported    soundPlanImportReport
		wantHeight  float64
		wantWarning bool
	}{
		{
			name:       "no layout falls back to RLKHEIGHT",
			meta:       soundplanimport.GridMapMetadata{ResultSubFolder: "RS01"},
			imported:   soundPlanImportReport{GridMapHeightM: 2},
			wantHeight: 2,
		},
		{
			name:       "no layout and no RLKHEIGHT falls back to the stated default",
			meta:       soundplanimport.GridMapMetadata{ResultSubFolder: "RS01"},
			imported:   soundPlanImportReport{},
			wantHeight: defaultGridMapReceiverHeightM,
		},
		{
			// Nothing places receivers at ground level, so a zero is a parse
			// artefact rather than a grid the comparison should honour.
			name:       "a zero height is not a height",
			meta:       referenceGridMapRun("RS01", false, 0),
			imported:   soundPlanImportReport{GridMapHeightM: 2},
			wantHeight: 2,
		},
		{
			// The bundle recorded no RLKHEIGHT, so there is nothing for the run
			// to contradict — warning against an assumed default would be noise.
			name:       "an unrecorded RLKHEIGHT cannot disagree",
			meta:       referenceGridMapRun("RS01", false, 2),
			imported:   soundPlanImportReport{},
			wantHeight: 2,
		},
		{
			name:       "agreement is silent",
			meta:       referenceGridMapRun("RS01", false, 2),
			imported:   soundPlanImportReport{GridMapHeightM: 2},
			wantHeight: 2,
		},
		{
			name:        "disagreement takes the run and says so",
			meta:        referenceGridMapRun("RS01", false, 4),
			imported:    soundPlanImportReport{GridMapHeightM: 2},
			wantHeight:  4,
			wantWarning: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			height, warnings := selectedRunReceiverHeight(testCase.meta, testCase.imported)
			if height != testCase.wantHeight {
				t.Fatalf("height = %v, want %v", height, testCase.wantHeight)
			}

			if got := len(warnings) > 0; got != testCase.wantWarning {
				t.Fatalf("warnings = %v, want a warning: %v", warnings, testCase.wantWarning)
			}
		})
	}
}

// syntheticRasterReceiverHeights reads the synthesized raster receivers' heights
// back out of the temporary model the preparation wrote, which is what the
// Aconiq run will compute at.
func syntheticRasterReceiverHeights(t *testing.T, prep *rasterComparePreparation) map[string]float64 {
	t.Helper()

	payload, err := os.ReadFile(prep.tempModelPath)
	if err != nil {
		t.Fatalf("read temp model: %v", err)
	}

	var collection struct {
		Features []struct {
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}

	if err := json.Unmarshal(payload, &collection); err != nil {
		t.Fatalf("decode temp model: %v", err)
	}

	out := make(map[string]float64)

	for _, feature := range collection.Features {
		id, _ := feature.Properties["id"].(string)
		if !strings.HasPrefix(id, soundPlanRasterReceiverPrefix) {
			continue
		}

		heightM, ok := feature.Properties["height_m"].(float64)
		if !ok {
			t.Fatalf("synthetic receiver %q carries no height_m", id)
		}

		out[id] = heightM
	}

	if len(out) == 0 {
		t.Fatal("the temporary model carries no synthetic raster receivers")
	}

	return out
}
