package soundplanimport

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/standards/schall03"
	absdb "github.com/cwbudde/go-absolute-database"
)

// Fixture-free tests for the staging loader and for the rail-operation
// derivation it calls. The project directory is assembled here from the
// synthetic images the other files in this package build, so the whole
// LoadProjectBundle path runs without the licensed project.

// bundleProjectSP enables one supported standard and one the importer has no
// mapping for, which is what makes the unsupported-standard warning reachable.
const bundleProjectSP = "[PROJECT]\n" +
	"TITLE=Bundle\n" +
	"[TIME SLICES DEN]\n" +
	"DAYIDENT=6-22\n" +
	"NIGHTIDENT=22-6\n" +
	"[ENABLEDSTANDARDS]\n" +
	"20490=1\n" +
	"99999=1\n"

// writeSyntheticProject lays out a SoundPLAN project directory holding one
// single-point run with rail tables, one grid-map run, and every optional
// geometry file the loader knows about. It returns the directory.
func writeSyntheticProject(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	writeFixture(t, dir, "Project.sp", []byte(bundleProjectSP))
	writeFixture(t, dir, "RSPS0011.res", []byte(
		"[General]\nResultSubFolder=RSPS0011\nRunType=Single Point\n"+
			"[GeoFiles]\nFileName0=GeoObjs.geo\nFileDate0=45000\n",
	))
	writeFixture(t, dir, "RRLK0022.res", []byte(
		"[General]\nResultSubFolder=RRLK0022\nRunType=Grid Map Sound\nRunCommands=GNM5:4\n"+
			"[Statistics]\nNoPointsTotal=6\n",
	))

	writeFixture(t, dir, "GeoObjs.geo", syntheticGeoObjs())
	writeFixture(t, dir, "GeoTmp.geo", syntheticGeoTmp())
	writeFixture(t, dir, "GeoWand.geo", concat(
		geoObjectRecord(wireTypeWall, 0),
		geoDataRecord('!', barrierAcousticsPayload(4, 0, 12), -1),
		geoPointRecord(10, 10, 215, 4),
		geoPointRecord(30, 10, 215, 4),
	))
	writeFixture(t, dir, "GeoRail.geo", concat(
		geoObjectRecord(0, 0),
		railNameRecord([]byte("Gleis 1"), -1),
		geoPointRecord(1000, 2000, 210, 209),
		geoPointRecord(1100, 2000, 210, 209),
		railParamsRecord(160, -1000, 0.5),
	))
	writeFixture(t, dir, "CalcArea.geo", concat(
		geoPointRecord(0, 0, 0, 0),
		geoPointRecord(100, 0, 0, 0),
		geoPointRecord(100, 80, 0, 0),
		geoPointRecord(0, 0, 0, 0),
	))
	writeFixture(t, dir, "IOTable1.ntd", ntdImage("StandardtextT", "No.", "RSPS0011.res", "RecNo)"))

	writeAbsTable(t, dir, "TS03.abs", "TS03",
		[]absdb.Column{absIntColumn("ZugArt"), absTextColumn("Name")},
		[][]any{{int32(8), "ICE"}})

	runDir := makeDir(t, dir, "RSPS0011")
	writeAbsTable(t, runDir, "RRAI0011.abs", "RRAI",
		[]absdb.Column{absIntColumn("IDX"), absIntColumn("ObjID"), absTextColumn("Railname")},
		[][]any{{int32(1), int32(42), "Gleis 1"}})
	writeAbsTable(t, runDir, "RRAD0011.abs", "RRAD",
		[]absdb.Column{absIntColumn("No"), absIntColumn("IDX"), absTextColumn("Trainname")},
		[][]any{{int32(1), int32(1), "ICE"}})

	gridDir := makeDir(t, dir, "RRLK0022")
	writeFixture(t, gridDir, "RRLK0022.GM", syntheticGridMap())

	return dir
}

// TestLoadProjectBundle_Synthetic pins that the staging loader collects every
// input it claims to, and that the one enabled standard the importer cannot
// map becomes a warning instead of a silent substitution.
func TestLoadProjectBundle_Synthetic(t *testing.T) {
	t.Parallel()

	dir := writeSyntheticProject(t)

	bundle, err := LoadProjectBundle(dir)
	if err != nil {
		t.Fatalf("LoadProjectBundle: %v", err)
	}

	if bundle.ProjectDir != dir {
		t.Errorf("ProjectDir = %q, want %q", bundle.ProjectDir, dir)
	}

	if bundle.Project == nil || bundle.Project.Title != "Bundle" {
		t.Fatalf("Project = %+v", bundle.Project)
	}

	if len(bundle.Runs) != 2 {
		t.Errorf("got %d runs, want 2", len(bundle.Runs))
	}

	if len(bundle.Standards) != 2 {
		t.Fatalf("Standards = %+v, want the two enabled ids", bundle.Standards)
	}

	if bundle.Standards[0].SoundPlanID != 20490 || !bundle.Standards[0].Supported {
		t.Errorf("Standards[0] = %+v, want the supported Schall 03 mapping", bundle.Standards[0])
	}

	if bundle.Standards[0].Aconiq.ID != schall03.Descriptor().ID {
		t.Errorf("mapped standard = %q, want %q", bundle.Standards[0].Aconiq.ID, schall03.Descriptor().ID)
	}

	if bundle.Standards[1].Supported {
		t.Errorf("Standards[1] = %+v, want the unknown id to be unsupported", bundle.Standards[1])
	}

	if !hasWarningContaining(bundle.Warnings, "standard 99999") {
		t.Errorf("Warnings = %v, want one naming the unsupported standard", bundle.Warnings)
	}

	if bundle.GeoObjects == nil || len(bundle.GeoObjects.Buildings) != 1 {
		t.Errorf("GeoObjects = %+v, want the building from GeoObjs.geo", bundle.GeoObjects)
	}

	if len(bundle.Barriers) != 1 || len(bundle.RailTracks) != 1 {
		t.Errorf("barriers/tracks = %d/%d, want 1/1", len(bundle.Barriers), len(bundle.RailTracks))
	}

	if bundle.CalcArea == nil || len(bundle.CalcArea.Points) != 4 {
		t.Errorf("CalcArea = %+v, want the four-point polygon", bundle.CalcArea)
	}

	if bundle.Terrain == nil || len(bundle.Terrain.ElevationPoints) != 1 {
		t.Errorf("Terrain = %+v, want the GeoTmp elevation sample", bundle.Terrain)
	}

	if len(bundle.ImmissionTables) != 1 || len(bundle.GridMaps) != 1 {
		t.Errorf("tables/grid maps = %d/%d, want 1/1", len(bundle.ImmissionTables), len(bundle.GridMaps))
	}

	if !bundle.GridMaps[0].DecodedValues {
		t.Errorf("grid map = %+v, want its payload decoded", bundle.GridMaps[0])
	}

	if len(bundle.TrainTypes) != 1 || bundle.TrainTypes[0].Name != "ICE" {
		t.Errorf("TrainTypes = %+v", bundle.TrainTypes)
	}

	if len(bundle.RailOps) != 1 || bundle.RailOps[0].Railname != "Gleis 1" {
		t.Errorf("RailOps = %+v", bundle.RailOps)
	}

	wantRefs := []string{"GeoObjs.geo", "RSPS0011"}
	for _, want := range wantRefs {
		if !hasWarningContaining(bundle.ResultFileRefs, want) {
			t.Errorf("ResultFileRefs = %v, want it to contain %q", bundle.ResultFileRefs, want)
		}
	}
}

// hasWarningContaining reports whether any entry contains the substring. It is
// used for both warnings and file references, which are free-text lists.
func hasWarningContaining(entries []string, want string) bool {
	for _, entry := range entries {
		if strings.Contains(entry, want) {
			return true
		}
	}

	return false
}

// TestLoadProjectBundle_MissingProjectFile pins the one hard failure: without
// Project.sp the directory is not a SoundPLAN project, and the loader must say
// so rather than return an empty bundle.
func TestLoadProjectBundle_MissingProjectFile(t *testing.T) {
	t.Parallel()

	_, err := LoadProjectBundle(t.TempDir())
	if err == nil {
		t.Fatal("got nil error for a directory with no Project.sp")
	}

	if !strings.Contains(err.Error(), "parse project") {
		t.Errorf("error = %q", err)
	}
}

// TestLoadProjectBundle_OptionalInputsDegradeToWarnings pins that every
// optional input is optional: a project with nothing but a Project.sp loads,
// and the inputs it lacks are reported rather than fabricated.
func TestLoadProjectBundle_OptionalInputsDegradeToWarnings(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFixture(t, dir, "Project.sp", []byte("[PROJECT]\nTITLE=Bare\n"))

	bundle, err := LoadProjectBundle(dir)
	if err != nil {
		t.Fatalf("LoadProjectBundle: %v", err)
	}

	if bundle.GeoObjects != nil || bundle.CalcArea != nil || bundle.Terrain != nil {
		t.Errorf("bundle = %+v, want no geometry, area or terrain", bundle)
	}

	if len(bundle.Runs) != 0 || len(bundle.RailOps) != 0 {
		t.Errorf("runs/rail ops = %d/%d, want none", len(bundle.Runs), len(bundle.RailOps))
	}

	if !hasWarningContaining(bundle.Warnings, "load terrain data") {
		t.Errorf("Warnings = %v, want the terrain failure reported", bundle.Warnings)
	}

	if !hasWarningContaining(bundle.Warnings, "no result subdirectory with both RRAI and RRAD") {
		t.Errorf("Warnings = %v, want the rail-operation failure reported", bundle.Warnings)
	}
}

// TestLoadProjectBundle_UnreadableGeometryBecomesAWarning pins that a geometry
// file that is present but will not parse names itself in the warnings, rather
// than disappearing into an absent-file branch.
func TestLoadProjectBundle_UnreadableGeometryBecomesAWarning(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFixture(t, dir, "Project.sp", []byte("[PROJECT]\nTITLE=Bare\n"))
	writeFixture(t, dir, "CalcArea.geo", []byte("no records here"))
	writeFixture(t, dir, "TS03.abs", []byte("not a database"))

	bundle, err := LoadProjectBundle(dir)
	if err != nil {
		t.Fatalf("LoadProjectBundle: %v", err)
	}

	if bundle.CalcArea != nil || len(bundle.TrainTypes) != 0 {
		t.Errorf("bundle = %+v, want neither input accepted", bundle)
	}

	for _, want := range []string{"CalcArea.geo", "TS03.abs"} {
		if !hasWarningContaining(bundle.Warnings, want) {
			t.Errorf("Warnings = %v, want one naming %q", bundle.Warnings, want)
		}
	}
}

// TestLoadRailOperationSummaries_Synthetic pins the derivation end to end: the
// RRAI row identifies the track, the RRAD rows linked by IDX give it its
// trains, and TS03 supplies the train class and traction the Schall 03 import
// needs.
func TestLoadRailOperationSummaries_Synthetic(t *testing.T) {
	t.Parallel()

	dir := writeSyntheticProject(t)

	proj, err := ParseProjectFile(filepath.Join(dir, "Project.sp"))
	if err != nil {
		t.Fatalf("ParseProjectFile: %v", err)
	}

	runs, err := ListRuns(dir)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}

	summaries, resultDir, err := LoadRailOperationSummaries(dir, proj, runs)
	if err != nil {
		t.Fatalf("LoadRailOperationSummaries: %v", err)
	}

	if filepath.Base(resultDir) != "RSPS0011" {
		t.Errorf("result dir = %q, want RSPS0011", resultDir)
	}

	if len(summaries) != 1 {
		t.Fatalf("got %d summaries, want 1", len(summaries))
	}

	summary := summaries[0]
	if summary.ObjID != 42 || summary.Railname != "Gleis 1" {
		t.Errorf("summary identity = %+v", summary)
	}

	if len(summary.TrainNames) != 1 || summary.TrainNames[0] != "ICE" {
		t.Errorf("TrainNames = %v, want [ICE]", summary.TrainNames)
	}

	if summary.DominantTrainName != "ICE" {
		t.Errorf("DominantTrainName = %q, want ICE", summary.DominantTrainName)
	}

	if summary.TrainClass != schall03.TrainClassPassenger {
		t.Errorf("TrainClass = %q, want %q", summary.TrainClass, schall03.TrainClassPassenger)
	}

	if summary.TractionType != schall03.TractionElectric {
		t.Errorf("TractionType = %q, want %q", summary.TractionType, schall03.TractionElectric)
	}

	if summary.AssessmentDayHours != 16 || summary.AssessmentNightHrs != 8 {
		t.Errorf("assessment hours = %v/%v, want 16/8", summary.AssessmentDayHours, summary.AssessmentNightHrs)
	}
}

// TestSelectRailOperationResultDir pins which run directory the derivation
// reads: a single-point run before a grid-map one, and only a directory that
// actually holds both tables.
func TestSelectRailOperationResultDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	for _, name := range []string{"RSPS0011", "RRLK0022", "RSPS0033"} {
		runDir := makeDir(t, dir, name)
		suffix := extractRunSuffix(name)

		if name == "RSPS0033" {
			// Only half the pair: this directory must not be chosen.
			writeFixture(t, runDir, "RRAI"+suffix+".abs", nil)

			continue
		}

		writeFixture(t, runDir, "RRAI"+suffix+".abs", nil)
		writeFixture(t, runDir, "RRAD"+suffix+".abs", nil)
	}

	tests := []struct {
		name    string
		runs    []*RunResult
		want    string
		wantErr bool
	}{
		{
			name: "a single-point run wins over a grid map listed before it",
			runs: []*RunResult{{ResultSubFolder: "RRLK0022"}, {ResultSubFolder: "RSPS0011"}},
			want: "RSPS0011",
		},
		{
			name: "a grid-map run is used when no single-point run has the tables",
			runs: []*RunResult{{ResultSubFolder: "RSPS0033"}, {ResultSubFolder: "RRLK0022"}},
			want: "RRLK0022",
		},
		{
			name:    "a run with only one of the two tables is not a candidate",
			runs:    []*RunResult{{ResultSubFolder: "RSPS0033"}},
			wantErr: true,
		},
		{
			name:    "a run whose subfolder is not a known kind is not a candidate",
			runs:    []*RunResult{{ResultSubFolder: "OTHER0011"}},
			wantErr: true,
		},
		{
			name:    "no runs at all",
			runs:    nil,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := selectRailOperationResultDir(dir, tc.runs)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("got %q, want a refusal", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("selectRailOperationResultDir: %v", err)
			}

			if filepath.Base(got) != tc.want {
				t.Errorf("selected %q, want %q", filepath.Base(got), tc.want)
			}
		})
	}
}

// TestDeriveAssessmentHours pins the day and night window lengths the rail
// import divides train counts by. A wrong answer here scales every derived
// trains-per-hour, so the defaults matter as much as the parsed values.
func TestDeriveAssessmentHours(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		proj      *Project
		wantDay   float64
		wantNight float64
	}{
		{name: "no project falls back to 16/8", proj: nil, wantDay: 16, wantNight: 8},
		{
			name:      "the usual German windows",
			proj:      &Project{DayPeriod: "6-22", NightPeriod: "22-6"},
			wantDay:   16,
			wantNight: 8,
		},
		{
			name:      "a window that wraps past midnight",
			proj:      &Project{DayPeriod: "7-23", NightPeriod: "23-7"},
			wantDay:   16,
			wantNight: 8,
		},
		{
			name:      "German decimals",
			proj:      &Project{DayPeriod: "6,5-22,5", NightPeriod: "22,5-6,5"},
			wantDay:   16,
			wantNight: 8,
		},
		{
			// SoundPLAN writes "0-0" for a window that is not used, and the
			// wrap-past-midnight rule turns that into a full day rather than
			// into the 16-hour default. Pinned as the shipped behaviour: a
			// project whose day window reads "0-0" divides its train counts by
			// 24, not by 16.
			name:      "an unused 0-0 window becomes a full day",
			proj:      &Project{DayPeriod: "0-0", NightPeriod: ""},
			wantDay:   24,
			wantNight: 8,
		},
		{
			name:      "a window longer than a day falls back",
			proj:      &Project{DayPeriod: "0-25", NightPeriod: "22-6"},
			wantDay:   16,
			wantNight: 8,
		},
		{
			name:      "a malformed window falls back",
			proj:      &Project{DayPeriod: "6-22-3", NightPeriod: "nonsense"},
			wantDay:   16,
			wantNight: 8,
		},
		{
			name:      "a short day window is used as declared",
			proj:      &Project{DayPeriod: "8-18", NightPeriod: "0-4"},
			wantDay:   10,
			wantNight: 4,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			day, night := deriveAssessmentHours(tc.proj)
			if day != tc.wantDay || night != tc.wantNight {
				t.Errorf("deriveAssessmentHours = %v/%v, want %v/%v", day, night, tc.wantDay, tc.wantNight)
			}
		})
	}
}

// TestBuildRailOperationSummary_NoTrains pins the branch a track with no train
// rows takes: the track's own maximum speed stands in for an average, and the
// class is left at mixed rather than guessed from nothing.
func TestBuildRailOperationSummary_NoTrains(t *testing.T) {
	t.Parallel()

	summary := buildRailOperationSummary(
		RailEmission{IDX: 1, ObjID: 7, Railname: " Gleis 3 ", TrackV: 120, DBue: 2},
		nil, nil, 16, 8,
	)

	if summary.Railname != "Gleis 3" || summary.ObjID != 7 {
		t.Errorf("summary = %+v", summary)
	}

	if summary.AverageSpeedKPH != 120 || summary.TrackVMaxKPH != 120 {
		t.Errorf("speeds = %v/%v, want the track maximum", summary.AverageSpeedKPH, summary.TrackVMaxKPH)
	}

	if !summary.OnBridge {
		t.Error("OnBridge = false although the bridge surcharge is positive")
	}

	if summary.TrainClass != schall03.TrainClassMixed || summary.TractionType != "" {
		t.Errorf("class/traction = %q/%q, want mixed and no traction", summary.TrainClass, summary.TractionType)
	}
}

// TestBuildRailOperationSummary_WeightedSpeedAndTraffic pins the two numbers
// the Schall 03 import actually consumes: the speed weighted by train count,
// and the trains per hour in each assessment window.
func TestBuildRailOperationSummary_WeightedSpeedAndTraffic(t *testing.T) {
	t.Parallel()

	linked := []TrainEmission{
		{IDX: 1, Trainname: "ICE", NDay: 24, NNight: 8, Speed: 200},
		{IDX: 1, Trainname: "RE", NDay: 8, NNight: 0, Speed: 100},
		{IDX: 1, Trainname: "Rangierfahrt", NDay: 0, NNight: 0, Speed: 40},
	}

	summary := buildRailOperationSummary(RailEmission{IDX: 1, ObjID: 1, TrackV: 160}, linked, nil, 16, 8)

	// (200*32 + 100*8) / 40 = 180; the zero-count train contributes nothing.
	if summary.AverageSpeedKPH != 180 {
		t.Errorf("AverageSpeedKPH = %v, want 180", summary.AverageSpeedKPH)
	}

	if summary.DayTrainCount != 32 || summary.NightTrainCount != 8 {
		t.Errorf("train counts = %v/%v, want 32/8", summary.DayTrainCount, summary.NightTrainCount)
	}

	if summary.TrafficDayPH != 2 || summary.TrafficNightPH != 1 {
		t.Errorf("trains per hour = %v/%v, want 2/1", summary.TrafficDayPH, summary.TrafficNightPH)
	}

	if summary.DominantTrainName != "ICE" {
		t.Errorf("DominantTrainName = %q, want the busiest train", summary.DominantTrainName)
	}
}
