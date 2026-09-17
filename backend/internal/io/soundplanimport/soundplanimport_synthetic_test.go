package soundplanimport

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fixture-free tests for the Project.sp and .res INI readers.
//
// Every fixture below is authored from the section and key names the parsers
// in soundplanimport.go declare, and from the German decimal convention their
// helpers document. No byte here is copied, trimmed or transcribed from the
// licensed SoundPLAN reference project, so all of this runs on a clean
// checkout — unlike the fixture-gated tests in soundplanimport_test.go, which
// skip in CI.

// writeFixture writes raw bytes into dir and returns the path.
//
// The bytes are written verbatim rather than through a string conversion so
// that a test can hand the parser a genuine Windows-1252 byte, which is the
// encoding SoundPLAN writes its INI files in.
func writeFixture(t *testing.T, dir string, name string, content []byte) string {
	t.Helper()

	path := filepath.Join(dir, name)

	err := os.WriteFile(path, content, 0o600)
	if err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}

	return path
}

// makeDir creates a subdirectory of parent and returns its path.
func makeDir(t *testing.T, parent string, name string) string {
	t.Helper()

	path := filepath.Join(parent, name)

	err := os.Mkdir(path, 0o750)
	if err != nil {
		t.Fatalf("mkdir %s: %v", name, err)
	}

	return path
}

// syntheticProjectSP is a complete Project.sp covering every section
// ParseProjectFile reads. TITLE carries the raw Windows-1252 byte 0xDF so that
// the decoder's job is visible: read as UTF-8 it would not be "ß".
const syntheticProjectSP = "[PROJECT]\r\n" +
	"TITLE=Stra\xdfe\r\n" +
	"VERSION=8\r\n" +
	"V64=1\r\n" +
	"\r\n" +
	"[TIME SLICES DEN]\r\n" +
	"DAYIDENT=6-22\r\n" +
	"EVENINGIDENT=0-0\r\n" +
	"NIGHTIDENT=22-6\r\n" +
	"ZBNAME1=Tag\r\n" +
	"ZBTYPE1=1\r\n" +
	"ZBISO1=59\r\n" +
	"ZBNAME2=Nacht\r\n" +
	"ZBTYPE2=1\r\n" +
	"ZBISO2=49\r\n" +
	"ZBNAME3=Kontur\r\n" +
	"ZBTYPE3=2\r\n" +
	"ZBISO3=70\r\n" +
	"ZBNAME4=\r\n" +
	"ZBTYPE4=1\r\n" +
	"ZBISO4=44\r\n" +
	"\r\n" +
	"[ENABLEDSTANDARDS]\r\n" +
	"20490=1\r\n" +
	"10490=0\r\n" +
	"30000=1\r\n" +
	"NOTANUMBER=1\r\n" +
	"\r\n" +
	"[ROAD]\r\n" +
	"SELECTED=10490\r\n" +
	"10490=road-parameters\r\n" +
	"\r\n" +
	"[RAIL]\r\n" +
	"SELECTED=20490\r\n" +
	"20490=rail-parameters\r\n" +
	"\r\n" +
	"[INDU]\r\n" +
	"SELECTED=0\r\n" +
	"\r\n" +
	"[SIMPLESETTINGS]\r\n" +
	"HABOVEGH=4,5\r\n" +
	"HABOVEEFH=1,8\r\n" +
	"FLOORHEIGHT=2,8\r\n" +
	"FLOORCOUNT=3\r\n" +
	"REFLORDNUNG=2\r\n" +
	"RAILBONUS=1\r\n" +
	"RLKHEIGHT=4,0\r\n" +
	"RLKDISTANCE=5,0\r\n" +
	"\r\n" +
	"[GEODB]\r\n" +
	"RELHEIGHTEFH=0,5\r\n" +
	"FLOORHEIGHT=2,9\r\n" +
	"RECFACDIST=0,1\r\n"

// TestParseProjectFile_SyntheticAllSections pins what every section of a
// Project.sp decodes to, including the three cases that must not produce an
// assessment period: a ZBTYPE other than 1, an empty ZBNAME, and a
// non-numeric key in [ENABLEDSTANDARDS].
func TestParseProjectFile_SyntheticAllSections(t *testing.T) {
	t.Parallel()

	path := writeFixture(t, t.TempDir(), "Project.sp", []byte(syntheticProjectSP))

	proj, err := ParseProjectFile(path)
	if err != nil {
		t.Fatalf("ParseProjectFile: %v", err)
	}

	if proj.Title != "Straße" {
		t.Errorf("Title = %q, want %q (Windows-1252 was not decoded)", proj.Title, "Straße")
	}

	if proj.Version != 8 || !proj.V64 {
		t.Errorf("Version/V64 = %d/%v, want 8/true", proj.Version, proj.V64)
	}

	if proj.DayPeriod != "6-22" || proj.EveningPeriod != "0-0" || proj.NightPeriod != "22-6" {
		t.Errorf("periods = %q/%q/%q, want 6-22/0-0/22-6", proj.DayPeriod, proj.EveningPeriod, proj.NightPeriod)
	}

	wantPeriods := []AssessmentPeriod{{Name: "Tag", ISOLevel: 59}, {Name: "Nacht", ISOLevel: 49}}
	if len(proj.AssessmentPeriods) != len(wantPeriods) {
		t.Fatalf("AssessmentPeriods = %+v, want %+v", proj.AssessmentPeriods, wantPeriods)
	}

	for i, want := range wantPeriods {
		if proj.AssessmentPeriods[i] != want {
			t.Errorf("AssessmentPeriods[%d] = %+v, want %+v", i, proj.AssessmentPeriods[i], want)
		}
	}

	wantStandards := map[int]bool{20490: true, 10490: false, 30000: true}
	if len(proj.EnabledStandards) != len(wantStandards) {
		t.Errorf("EnabledStandards = %v, want %v (a non-numeric key must be skipped)", proj.EnabledStandards, wantStandards)
	}

	for id, want := range wantStandards {
		if proj.EnabledStandards[id] != want {
			t.Errorf("EnabledStandards[%d] = %v, want %v", id, proj.EnabledStandards[id], want)
		}
	}

	if proj.RoadParams != (NoiseTypeParams{SelectedStandard: 10490, ParamString: "road-parameters"}) {
		t.Errorf("RoadParams = %+v", proj.RoadParams)
	}

	if proj.RailParams != (NoiseTypeParams{SelectedStandard: 20490, ParamString: "rail-parameters"}) {
		t.Errorf("RailParams = %+v", proj.RailParams)
	}

	if proj.InduParams != (NoiseTypeParams{}) {
		t.Errorf("InduParams = %+v, want the zero value for SELECTED=0 with no matching key", proj.InduParams)
	}

	wantSettings := ProjectSettings{
		ReceiverHeightAboveGround: 4.5,
		ReceiverHeightAboveFloor:  1.8,
		FloorHeight:               2.8,
		FloorCount:                3,
		ReflectionOrder:           2,
		RailBonus:                 true,
		GridMapHeight:             4,
		GridMapDistance:           5,
	}
	if proj.Settings != wantSettings {
		t.Errorf("Settings = %+v, want %+v", proj.Settings, wantSettings)
	}

	wantGeoDB := GeoDBDefaults{RelHeightEFH: 0.5, FloorHeight: 2.9, ReceiverFacadeDistance: 0.1}
	if proj.GeoDB != wantGeoDB {
		t.Errorf("GeoDB = %+v, want %+v", proj.GeoDB, wantGeoDB)
	}
}

// TestParseProjectFile_MalformedINI pins what the INI reader drops rather than
// guesses at. Each case is a whole file; the parser must return a usable
// Project in every one of them, because a SoundPLAN project with one odd line
// is still a project.
func TestParseProjectFile_MalformedINI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		check   func(t *testing.T, proj *Project)
	}{
		{
			name:    "key before any section is dropped",
			content: "TITLE=orphan\n[PROJECT]\nVERSION=3\n",
			check: func(t *testing.T, proj *Project) {
				t.Helper()

				if proj.Title != "" {
					t.Errorf("Title = %q, want empty: the key sits outside any section", proj.Title)
				}

				if proj.Version != 3 {
					t.Errorf("Version = %d, want 3", proj.Version)
				}
			},
		},
		{
			name:    "unterminated section header does not open a section",
			content: "[PROJECT\nTITLE=lost\n",
			check: func(t *testing.T, proj *Project) {
				t.Helper()

				if proj.Title != "" {
					t.Errorf("Title = %q, want empty: '[PROJECT' never closed", proj.Title)
				}
			},
		},
		{
			name:    "trailing junk after the closing bracket is ignored",
			content: "[PROJECT] ; comment\nTITLE=kept\n",
			check: func(t *testing.T, proj *Project) {
				t.Helper()

				if proj.Title != "kept" {
					t.Errorf("Title = %q, want %q", proj.Title, "kept")
				}
			},
		},
		{
			name:    "line without a separator is skipped",
			content: "[PROJECT]\nthis line has no equals sign\nTITLE=kept\n",
			check: func(t *testing.T, proj *Project) {
				t.Helper()

				if proj.Title != "kept" {
					t.Errorf("Title = %q, want %q", proj.Title, "kept")
				}
			},
		},
		{
			name:    "a repeated key keeps the last value",
			content: "[PROJECT]\nTITLE=first\nTITLE=last\n",
			check: func(t *testing.T, proj *Project) {
				t.Helper()

				if proj.Title != "last" {
					t.Errorf("Title = %q, want %q", proj.Title, "last")
				}
			},
		},
		{
			name:    "a section repeated later adds to the same map",
			content: "[PROJECT]\nTITLE=a\n[GEODB]\nRECFACDIST=0,25\n[PROJECT]\nVERSION=9\n",
			check: func(t *testing.T, proj *Project) {
				t.Helper()

				if proj.Title != "a" || proj.Version != 9 {
					t.Errorf("Title/Version = %q/%d, want a/9", proj.Title, proj.Version)
				}

				if proj.GeoDB.ReceiverFacadeDistance != 0.25 {
					t.Errorf("RECFACDIST = %v, want 0.25", proj.GeoDB.ReceiverFacadeDistance)
				}
			},
		},
		{
			name:    "an unparsable number reads as zero rather than failing the file",
			content: "[PROJECT]\nVERSION=acht\nV64=0\n[SIMPLESETTINGS]\nHABOVEGH=vier\nFLOORCOUNT=\n",
			check: func(t *testing.T, proj *Project) {
				t.Helper()

				if proj.Version != 0 || proj.V64 {
					t.Errorf("Version/V64 = %d/%v, want 0/false", proj.Version, proj.V64)
				}

				if proj.Settings.ReceiverHeightAboveGround != 0 || proj.Settings.FloorCount != 0 {
					t.Errorf("Settings = %+v, want zeroes", proj.Settings)
				}
			},
		},
		{
			name:    "an empty value is kept as an empty string",
			content: "[PROJECT]\nTITLE=\n",
			check: func(t *testing.T, proj *Project) {
				t.Helper()

				if proj.Title != "" {
					t.Errorf("Title = %q, want empty", proj.Title)
				}
			},
		},
		{
			name:    "a file with no recognised section yields an empty project",
			content: "[UNRELATED]\nANYTHING=1\n",
			check: func(t *testing.T, proj *Project) {
				t.Helper()

				if proj.Title != "" || proj.Version != 0 || len(proj.EnabledStandards) != 0 {
					t.Errorf("proj = %+v, want an empty project", proj)
				}

				if proj.EnabledStandards == nil {
					t.Error("EnabledStandards must be an allocated map even when the section is absent")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := writeFixture(t, t.TempDir(), "Project.sp", []byte(tc.content))

			proj, err := ParseProjectFile(path)
			if err != nil {
				t.Fatalf("ParseProjectFile: %v", err)
			}

			tc.check(t, proj)
		})
	}
}

// TestParseProjectFile_MissingFile pins that an absent Project.sp is an error
// and not an empty project: LoadProjectBundle uses it to decide whether the
// directory is a SoundPLAN project at all.
func TestParseProjectFile_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := ParseProjectFile(filepath.Join(t.TempDir(), "Project.sp"))
	if err == nil {
		t.Fatal("ParseProjectFile on a missing file: got nil error, want a failure")
	}

	if !strings.Contains(err.Error(), "soundplan: parse project") {
		t.Errorf("error = %q, want it to name the parse step", err)
	}
}

// TestParseINIFile_OverlongLineIsRefused pins the scanner's 1 MiB line ceiling.
//
// bufio.Scanner reports bufio.ErrTooLong for a longer line, and parseINIFile
// must surface it. Returning the sections read so far would present a
// truncated project as a complete one.
func TestParseINIFile_OverlongLineIsRefused(t *testing.T) {
	t.Parallel()

	content := "[PROJECT]\nTITLE=" + strings.Repeat("x", 1024*1024+1) + "\n"

	_, err := ParseProjectFile(writeFixture(t, t.TempDir(), "Project.sp", []byte(content)))
	if err == nil {
		t.Fatal("got nil error for a line past the scanner buffer limit, want a failure")
	}

	if !strings.Contains(err.Error(), "scan") {
		t.Errorf("error = %q, want it to name the scan failure", err)
	}
}

// syntheticResFile exercises every section ParseResFile reads, and three
// boundaries at once: a gap in the FileName index, a declared warning count
// larger than the non-empty warnings present, and an hour mask with more than
// 24 fields.
const syntheticResFile = "[General]\n" +
	"RKVersion=8.0\n" +
	"Productversion=8.0.0.0\n" +
	"ResultSubFolder=RSPS0011\n" +
	"RunType=Single Point\n" +
	"RunStart=2024-01-02 10:00:00\n" +
	"RunStop=2024-01-02 10:00:41\n" +
	"RunCommands=GNM5:4\n" +
	"RunData=\"\"GeoObjs.geo\" \"GeoRail.geo\"\"\n" +
	"ThreadCount=4\n" +
	"Error=0\n" +
	"SourceTypes=2\n" +
	"\n" +
	"[Comments]\n" +
	"Warnings=3\n" +
	"Warning1=first warning\n" +
	"Warning2=\n" +
	"Warning3=third warning\n" +
	"\n" +
	"[Statistics]\n" +
	"CalcTime=41000\n" +
	"NoPointsTotal=30\n" +
	"NoPointsCalculated=29\n" +
	"\n" +
	"[GeoFiles]\n" +
	"FileName0=GeoObjs.geo\n" +
	"FileDate0=45000\n" +
	"FileName1=GeoRail.geo\n" +
	"FileDate1=not-a-date\n" +
	"FileName3=NeverRead.geo\n" +
	"FileDate3=45002\n" +
	"\n" +
	"[Assessment.ZB1]\n" +
	"ZBName=Tag\n" +
	"AssessType=Mittelungspegel\n" +
	"TAssess=16\n" +
	"Hours=0,0,0,0,0,0,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,0,0\n" +
	"\n" +
	"[Assessment.ZB2]\n" +
	"ZBName=\n" +
	"TAssess=0\n" +
	"\n" +
	"[Assessment.ZB3]\n" +
	"ZBName=Nacht\n" +
	"AssessType=Mittelungspegel\n" +
	"TAssess=8,5\n" +
	"Hours=1,1,1,1,1,1,0, 0 ,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,1,1,1,1,1\n"

// TestParseResFile_Synthetic pins the .res reader field by field.
func TestParseResFile_Synthetic(t *testing.T) {
	t.Parallel()

	res, err := ParseResFile(writeFixture(t, t.TempDir(), "RSPS0011.res", []byte(syntheticResFile)))
	if err != nil {
		t.Fatalf("ParseResFile: %v", err)
	}

	if res.RKVersion != "8.0" || res.ProductVersion != "8.0.0.0" {
		t.Errorf("versions = %q/%q, want 8.0/8.0.0.0", res.RKVersion, res.ProductVersion)
	}

	if res.ResultSubFolder != "RSPS0011" || res.RunType != "Single Point" {
		t.Errorf("subfolder/type = %q/%q", res.ResultSubFolder, res.RunType)
	}

	if res.RunStart != "2024-01-02 10:00:00" || res.RunStop != "2024-01-02 10:00:41" {
		t.Errorf("run window = %q..%q", res.RunStart, res.RunStop)
	}

	if res.ThreadCount != 4 || res.ErrorCode != 0 || res.SourceTypes != 2 {
		t.Errorf("threads/error/sourcetypes = %d/%d/%d, want 4/0/2", res.ThreadCount, res.ErrorCode, res.SourceTypes)
	}

	wantStats := RunStatistics{CalcTimeMS: 41000, PointsTotal: 30, PointsCalculated: 29}
	if res.Statistics != wantStats {
		t.Errorf("Statistics = %+v, want %+v", res.Statistics, wantStats)
	}

	wantWarnings := []string{"first warning", "third warning"}
	if len(res.Warnings) != len(wantWarnings) {
		t.Fatalf("Warnings = %v, want %v (an empty Warning<n> must not be collected)", res.Warnings, wantWarnings)
	}

	for i, want := range wantWarnings {
		if res.Warnings[i] != want {
			t.Errorf("Warnings[%d] = %q, want %q", i, res.Warnings[i], want)
		}
	}

	// FileName2 is absent, so the enumeration stops there and FileName3 is
	// never read. Anything else would silently renumber the run's inputs.
	wantFiles := []GeoFileRef{{Name: "GeoObjs.geo", Date: 45000}, {Name: "GeoRail.geo", Date: 0}}
	if len(res.GeoFiles) != len(wantFiles) {
		t.Fatalf("GeoFiles = %+v, want %+v", res.GeoFiles, wantFiles)
	}

	for i, want := range wantFiles {
		if res.GeoFiles[i] != want {
			t.Errorf("GeoFiles[%d] = %+v, want %+v", i, res.GeoFiles[i], want)
		}
	}
}

// TestParseResFile_AssessmentPeriods pins the assessment-window decoding: a
// section whose ZBName is empty is skipped, a gap in the ZB numbering does not
// stop the scan, and an hour mask is read as exactly 24 bits.
func TestParseResFile_AssessmentPeriods(t *testing.T) {
	t.Parallel()

	res, err := ParseResFile(writeFixture(t, t.TempDir(), "RSPS0011.res", []byte(syntheticResFile)))
	if err != nil {
		t.Fatalf("ParseResFile: %v", err)
	}

	if len(res.AssessmentPeriods) != 2 {
		t.Fatalf("AssessmentPeriods = %+v, want the two named windows", res.AssessmentPeriods)
	}

	day := res.AssessmentPeriods[0]
	if day.Name != "Tag" || day.AssessType != "Mittelungspegel" || day.AssessHours != 16 {
		t.Errorf("day window = %+v", day)
	}

	for hour := range 24 {
		want := hour >= 6 && hour < 22
		if day.Hours[hour] != want {
			t.Errorf("day hour %d = %v, want %v", hour, day.Hours[hour], want)
		}
	}

	night := res.AssessmentPeriods[1]
	if night.Name != "Nacht" {
		t.Errorf("second window = %q, want Nacht (ZB2 has no name and ZB3 must still be read)", night.Name)
	}

	// The mask declares 28 fields; only the first 24 may reach the array, and
	// the surplus must not panic or wrap around.
	for hour := range 24 {
		want := hour < 6 || hour >= 22
		if night.Hours[hour] != want {
			t.Errorf("night hour %d = %v, want %v", hour, night.Hours[hour], want)
		}
	}

	// TAssess is read with strconv.ParseFloat and not with parseGermanFloat,
	// so the German decimal "8,5" does not survive. Pinned as the behaviour
	// that is actually shipped, not as the behaviour one would want.
	if night.AssessHours != 0 {
		t.Errorf("night AssessHours = %v, want 0 for the German decimal 8,5", night.AssessHours)
	}
}

// TestParseResFile_MissingSectionsAreEmpty pins that a .res holding only a
// [General] section parses into a run with no statistics, files or windows,
// rather than into an error.
func TestParseResFile_MissingSectionsAreEmpty(t *testing.T) {
	t.Parallel()

	content := "[General]\nRunType=Grid Map Sound\nResultSubFolder=RRLK0022\n"

	res, err := ParseResFile(writeFixture(t, t.TempDir(), "RRLK0022.res", []byte(content)))
	if err != nil {
		t.Fatalf("ParseResFile: %v", err)
	}

	if res.RunType != "Grid Map Sound" || res.ResultSubFolder != "RRLK0022" {
		t.Errorf("res = %+v", res)
	}

	if len(res.GeoFiles) != 0 || len(res.Warnings) != 0 || len(res.AssessmentPeriods) != 0 {
		t.Errorf("expected no files, warnings or windows, got %+v", res)
	}

	if res.RunData != "" || res.RunDataFiles != nil {
		t.Errorf("RunData = %q / %v, want empty", res.RunData, res.RunDataFiles)
	}

	if _, ok := res.GridMapLayout(); ok {
		t.Error("GridMapLayout reported a layout for a run with no RunCommands")
	}
}

// TestListRuns_Synthetic pins the discovery step: every *.res in the directory
// is parsed, files with other extensions are ignored, and the results keep the
// glob's sorted order.
func TestListRuns_Synthetic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFixture(t, dir, "RSPS0011.res", []byte("[General]\nResultSubFolder=RSPS0011\n"))
	writeFixture(t, dir, "RRLK0022.res", []byte("[General]\nResultSubFolder=RRLK0022\n"))
	writeFixture(t, dir, "Project.sp", []byte("[PROJECT]\nTITLE=x\n"))

	runs, err := ListRuns(dir)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}

	want := []string{"RRLK0022", "RSPS0011"}
	if len(runs) != len(want) {
		t.Fatalf("got %d runs, want %d", len(runs), len(want))
	}

	for i, name := range want {
		if runs[i].ResultSubFolder != name {
			t.Errorf("runs[%d] = %q, want %q", i, runs[i].ResultSubFolder, name)
		}
	}
}

// TestListRuns_EmptyDirectory pins that a directory without result files is
// not an error: LoadProjectBundle calls this before it knows whether the
// project was ever calculated.
func TestListRuns_EmptyDirectory(t *testing.T) {
	t.Parallel()

	runs, err := ListRuns(t.TempDir())
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}

	if len(runs) != 0 {
		t.Errorf("got %d runs, want none", len(runs))
	}
}

// TestListRuns_UnreadableEntryFails pins that ListRuns surfaces a per-file
// failure with the file's name instead of dropping the run. A directory whose
// name ends in .res cannot be read as a file, which is the cheapest way to
// produce that failure without a permission trick.
func TestListRuns_UnreadableEntryFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := os.Mkdir(filepath.Join(dir, "RSPS0011.res"), 0o750)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err = ListRuns(dir)
	if err == nil {
		t.Fatal("ListRuns: got nil error for an unreadable .res entry")
	}

	if !strings.Contains(err.Error(), "RSPS0011.res") {
		t.Errorf("error = %q, want it to name the offending file", err)
	}
}

// TestGridMapLayoutRejectsNonFiniteHeight pins parseDecimal's finiteness
// guard: a run whose height overflows float64 must report "no layout" rather
// than an infinite receiver height.
func TestGridMapLayoutRejectsNonFiniteHeight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		runCommands string
		wantOK      bool
	}{
		{name: "finite", runCommands: "GNM5:4", wantOK: true},
		{name: "german decimal", runCommands: "GNM2,5:3,5", wantOK: true},
		{name: "height overflows float64", runCommands: "GNM5:1" + strings.Repeat("0", 400), wantOK: false},
		{name: "spacing overflows float64", runCommands: "GNM1" + strings.Repeat("0", 400) + ":4", wantOK: false},
		{name: "zero spacing", runCommands: "GNM0:4", wantOK: false},
		{name: "no separator", runCommands: "GNM54", wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res := &RunResult{RunCommands: tc.runCommands}

			layout, ok := res.GridMapLayout()
			if ok != tc.wantOK {
				t.Fatalf("GridMapLayout(%q) ok = %v, want %v", tc.runCommands, ok, tc.wantOK)
			}

			if ok && (math.IsInf(layout.HeightM, 0) || math.IsInf(layout.SpacingM, 0)) {
				t.Errorf("GridMapLayout(%q) = %+v, which is not finite", tc.runCommands, layout)
			}
		})
	}
}
