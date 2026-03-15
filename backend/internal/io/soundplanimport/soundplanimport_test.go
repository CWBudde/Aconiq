package soundplanimport

import (
	"os"
	"path/filepath"
	"testing"
)

// testProjectDir returns the path to the sample SoundPlan project.
// Tests that need the real project files are skipped if the directory
// does not exist (e.g. in CI without the test data).
func testProjectDir(t *testing.T) string {
	t.Helper()

	dir := filepath.Join("..", "..", "..", "..", "interoperability", "Schienenprojekt - Schall 03")

	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		t.Skip("sample SoundPlan project not available")
	}

	return dir
}

// ---------------------------------------------------------------------------
// Project.sp parsing
// ---------------------------------------------------------------------------

func TestParseProjectSP_Metadata(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	proj, err := ParseProjectFile(filepath.Join(dir, "Project.sp"))
	if err != nil {
		t.Fatalf("ParseProjectFile: %v", err)
	}

	if proj.Version != 41080 {
		t.Errorf("Version = %d, want 41080", proj.Version)
	}

	if proj.Title == "" {
		t.Error("Title is empty, want non-empty project title")
	}

	if proj.V64 != true {
		t.Error("V64 = false, want true (64-bit project)")
	}
}

func TestParseProjectSP_TimePeriods(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	proj, err := ParseProjectFile(filepath.Join(dir, "Project.sp"))
	if err != nil {
		t.Fatalf("ParseProjectFile: %v", err)
	}

	if proj.DayPeriod != "6-22" {
		t.Errorf("DayPeriod = %q, want %q", proj.DayPeriod, "6-22")
	}

	if proj.NightPeriod != "22-6" {
		t.Errorf("NightPeriod = %q, want %q", proj.NightPeriod, "22-6")
	}

	if proj.EveningPeriod != "0-0" {
		t.Errorf("EveningPeriod = %q, want %q", proj.EveningPeriod, "0-0")
	}

	// Check named assessment periods.
	if len(proj.AssessmentPeriods) < 2 {
		t.Fatalf("AssessmentPeriods has %d entries, want >= 2", len(proj.AssessmentPeriods))
	}

	tag := proj.AssessmentPeriods[0]
	if tag.Name != "Tag" {
		t.Errorf("AssessmentPeriods[0].Name = %q, want %q", tag.Name, "Tag")
	}

	if tag.ISOLevel != 72 {
		t.Errorf("AssessmentPeriods[0].ISOLevel = %d, want 72", tag.ISOLevel)
	}

	nacht := proj.AssessmentPeriods[1]
	if nacht.Name != "Nacht" {
		t.Errorf("AssessmentPeriods[1].Name = %q, want %q", nacht.Name, "Nacht")
	}

	if nacht.ISOLevel != 62 {
		t.Errorf("AssessmentPeriods[1].ISOLevel = %d, want 62", nacht.ISOLevel)
	}
}

func TestParseProjectSP_EnabledStandards(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	proj, err := ParseProjectFile(filepath.Join(dir, "Project.sp"))
	if err != nil {
		t.Fatalf("ParseProjectFile: %v", err)
	}

	// Schall 03 rail standard (20490) must be enabled.
	if !proj.EnabledStandards[20490] {
		t.Error("standard 20490 (Schall 03 rail) not enabled")
	}

	// RLS-19 road standard (10490) must be enabled.
	if !proj.EnabledStandards[10490] {
		t.Error("standard 10490 (RLS-19 road) not enabled")
	}

	// A disabled standard should be false.
	if proj.EnabledStandards[10440] {
		t.Error("standard 10440 should be disabled")
	}
}

func TestParseProjectSP_CalcParams(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	proj, err := ParseProjectFile(filepath.Join(dir, "Project.sp"))
	if err != nil {
		t.Fatalf("ParseProjectFile: %v", err)
	}

	rail := proj.RailParams
	if rail.SelectedStandard != 20490 {
		t.Errorf("RailParams.SelectedStandard = %d, want 20490", rail.SelectedStandard)
	}

	if rail.ParamString == "" {
		t.Error("RailParams.ParamString is empty")
	}

	road := proj.RoadParams
	if road.SelectedStandard != 10490 {
		t.Errorf("RoadParams.SelectedStandard = %d, want 10490", road.SelectedStandard)
	}

	indu := proj.InduParams
	if indu.SelectedStandard != 30000 {
		t.Errorf("InduParams.SelectedStandard = %d, want 30000", indu.SelectedStandard)
	}
}

func TestParseProjectSP_Settings(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	proj, err := ParseProjectFile(filepath.Join(dir, "Project.sp"))
	if err != nil {
		t.Fatalf("ParseProjectFile: %v", err)
	}

	if proj.Settings.ReceiverHeightAboveGround != 2.0 {
		t.Errorf("ReceiverHeightAboveGround = %f, want 2.0", proj.Settings.ReceiverHeightAboveGround)
	}

	if proj.Settings.ReceiverHeightAboveFloor != 2.4 {
		t.Errorf("ReceiverHeightAboveFloor = %f, want 2.4", proj.Settings.ReceiverHeightAboveFloor)
	}

	if proj.Settings.FloorHeight != 2.8 {
		t.Errorf("FloorHeight = %f, want 2.8", proj.Settings.FloorHeight)
	}

	if proj.Settings.FloorCount != 3 {
		t.Errorf("FloorCount = %d, want 3", proj.Settings.FloorCount)
	}

	if proj.Settings.ReflectionOrder != 1 {
		t.Errorf("ReflectionOrder = %d, want 1", proj.Settings.ReflectionOrder)
	}

	if proj.Settings.RailBonus != true {
		t.Error("RailBonus = false, want true")
	}

	if proj.Settings.GridMapHeight != 2.0 {
		t.Errorf("GridMapHeight = %f, want 2.0", proj.Settings.GridMapHeight)
	}

	if proj.Settings.GridMapDistance != 5.0 {
		t.Errorf("GridMapDistance = %f, want 5.0", proj.Settings.GridMapDistance)
	}
}

func TestParseProjectSP_GeoDefaults(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	proj, err := ParseProjectFile(filepath.Join(dir, "Project.sp"))
	if err != nil {
		t.Fatalf("ParseProjectFile: %v", err)
	}

	if proj.GeoDB.ReceiverFacadeDistance != 0.01 {
		t.Errorf("ReceiverFacadeDistance = %f, want 0.01", proj.GeoDB.ReceiverFacadeDistance)
	}

	if proj.GeoDB.FloorHeight != 2.8 {
		t.Errorf("GeoDB.FloorHeight = %f, want 2.8", proj.GeoDB.FloorHeight)
	}

	if proj.GeoDB.RelHeightEFH != 2.4 {
		t.Errorf("RelHeightEFH = %f, want 2.4", proj.GeoDB.RelHeightEFH)
	}
}

// ---------------------------------------------------------------------------
// .res file parsing
// ---------------------------------------------------------------------------

func TestParseResFile_RunMetadata(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	res, err := ParseResFile(filepath.Join(dir, "RSPS0011.res"))
	if err != nil {
		t.Fatalf("ParseResFile: %v", err)
	}

	if res.RunType != "Single Point Sound" {
		t.Errorf("RunType = %q, want %q", res.RunType, "Single Point Sound")
	}

	if res.ResultSubFolder != "RSPS0011" {
		t.Errorf("ResultSubFolder = %q, want %q", res.ResultSubFolder, "RSPS0011")
	}

	if res.ThreadCount != 8 {
		t.Errorf("ThreadCount = %d, want 8", res.ThreadCount)
	}

	if res.ErrorCode != 0 {
		t.Errorf("ErrorCode = %d, want 0", res.ErrorCode)
	}

	if res.SourceTypes != 4 {
		t.Errorf("SourceTypes = %d, want 4", res.SourceTypes)
	}
}

func TestParseResFile_Statistics(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	res, err := ParseResFile(filepath.Join(dir, "RSPS0011.res"))
	if err != nil {
		t.Fatalf("ParseResFile: %v", err)
	}

	if res.Statistics.CalcTimeMS != 2498 {
		t.Errorf("CalcTimeMS = %d, want 2498", res.Statistics.CalcTimeMS)
	}

	if res.Statistics.PointsTotal != 13 {
		t.Errorf("PointsTotal = %d, want 13", res.Statistics.PointsTotal)
	}

	if res.Statistics.PointsCalculated != 13 {
		t.Errorf("PointsCalculated = %d, want 13", res.Statistics.PointsCalculated)
	}
}

func TestParseResFile_GeoFiles(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	res, err := ParseResFile(filepath.Join(dir, "RSPS0011.res"))
	if err != nil {
		t.Fatalf("ParseResFile: %v", err)
	}

	if len(res.GeoFiles) != 3 {
		t.Fatalf("GeoFiles has %d entries, want 3", len(res.GeoFiles))
	}

	names := make(map[string]bool)
	for _, gf := range res.GeoFiles {
		names[gf.Name] = true
	}

	for _, want := range []string{"GeoObjs.geo", "GeoRail.geo", "RDGM0001.dgm"} {
		if !names[want] {
			t.Errorf("GeoFiles missing %q", want)
		}
	}
}

func TestParseResFile_Assessment(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	res, err := ParseResFile(filepath.Join(dir, "RSPS0011.res"))
	if err != nil {
		t.Fatalf("ParseResFile: %v", err)
	}

	if len(res.AssessmentPeriods) < 2 {
		t.Fatalf("AssessmentPeriods has %d entries, want >= 2", len(res.AssessmentPeriods))
	}

	tag := res.AssessmentPeriods[0]
	if tag.Name != "Tag" {
		t.Errorf("AssessmentPeriods[0].Name = %q, want %q", tag.Name, "Tag")
	}

	if tag.AssessType != "Leq" {
		t.Errorf("AssessmentPeriods[0].AssessType = %q, want %q", tag.AssessType, "Leq")
	}

	if tag.AssessHours != 16.0 {
		t.Errorf("AssessmentPeriods[0].AssessHours = %f, want 16.0", tag.AssessHours)
	}

	nacht := res.AssessmentPeriods[1]
	if nacht.Name != "Nacht" {
		t.Errorf("AssessmentPeriods[1].Name = %q, want %q", nacht.Name, "Nacht")
	}

	if nacht.AssessHours != 8.0 {
		t.Errorf("AssessmentPeriods[1].AssessHours = %f, want 8.0", nacht.AssessHours)
	}
}

func TestParseResFile_Warnings(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	res, err := ParseResFile(filepath.Join(dir, "RSPS0011.res"))
	if err != nil {
		t.Fatalf("ParseResFile: %v", err)
	}

	if len(res.Warnings) != 1 {
		t.Fatalf("Warnings has %d entries, want 1", len(res.Warnings))
	}

	if res.Warnings[0] == "" {
		t.Error("Warning[0] is empty")
	}
}

func TestParseResFile_GridMap(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	res, err := ParseResFile(filepath.Join(dir, "RRLK0022.res"))
	if err != nil {
		t.Fatalf("ParseResFile: %v", err)
	}

	if res.RunType != "Grid Map Sound" {
		t.Errorf("RunType = %q, want %q", res.RunType, "Grid Map Sound")
	}

	if res.Statistics.PointsTotal != 5961 {
		t.Errorf("PointsTotal = %d, want 5961", res.Statistics.PointsTotal)
	}
}

// ---------------------------------------------------------------------------
// ListRuns discovery
// ---------------------------------------------------------------------------

func TestListRuns(t *testing.T) {
	t.Parallel()

	dir := testProjectDir(t)

	runs, err := ListRuns(dir)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}

	if len(runs) == 0 {
		t.Fatal("ListRuns returned no runs")
	}

	// The sample project has RSPS and RRLK runs.
	hasSpS := false
	hasRLK := false

	for _, r := range runs {
		switch r.RunType {
		case "Single Point Sound":
			hasSpS = true
		case "Grid Map Sound":
			hasRLK = true
		}
	}

	if !hasSpS {
		t.Error("no Single Point Sound run found")
	}

	if !hasRLK {
		t.Error("no Grid Map Sound run found")
	}
}
