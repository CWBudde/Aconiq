// Package soundplanimport reads SoundPlan project files (.sp) and associated
// result metadata (.res) into structured Go types for cross-validation.
package soundplanimport

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// Project holds the parsed contents of a SoundPlan Project.sp file.
type Project struct {
	Title   string
	Version int
	V64     bool

	DayPeriod     string // e.g. "6-22"
	EveningPeriod string // e.g. "0-0"
	NightPeriod   string // e.g. "22-6"

	AssessmentPeriods []AssessmentPeriod

	EnabledStandards map[int]bool // standard ID → enabled

	RoadParams NoiseTypeParams
	RailParams NoiseTypeParams
	InduParams NoiseTypeParams

	Settings ProjectSettings
	GeoDB    GeoDBDefaults
}

// AssessmentPeriod defines a named time-of-day assessment window.
type AssessmentPeriod struct {
	Name     string
	ISOLevel int // dB threshold for ISO contour (ZBISO)
}

// NoiseTypeParams holds the selected standard and parameter string
// for a noise type (road/rail/industry).
type NoiseTypeParams struct {
	SelectedStandard int
	ParamString      string
}

// ProjectSettings holds calculation settings from [SIMPLESETTINGS].
type ProjectSettings struct {
	ReceiverHeightAboveGround float64
	ReceiverHeightAboveFloor  float64
	FloorHeight               float64
	FloorCount                int
	ReflectionOrder           int
	RailBonus                 bool
	GridMapHeight             float64
	GridMapDistance           float64
}

// GeoDBDefaults holds geometry database defaults from [GEODB].
type GeoDBDefaults struct {
	RelHeightEFH           float64
	FloorHeight            float64
	ReceiverFacadeDistance float64
}

// ParseProjectFile reads and parses a SoundPlan Project.sp file.
func ParseProjectFile(path string) (*Project, error) {
	sections, err := parseINIFile(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: parse project: %w", err)
	}

	proj := &Project{
		EnabledStandards: make(map[int]bool),
	}

	parseProjectSection(proj, sections)
	parseTimeSlices(proj, sections)
	parseEnabledStandards(proj, sections)

	proj.RoadParams = parseNoiseTypeSection(sections, "ROAD")
	proj.RailParams = parseNoiseTypeSection(sections, "RAIL")
	proj.InduParams = parseNoiseTypeSection(sections, "INDU")

	parseSimpleSettings(proj, sections)
	parseGeoDB(proj, sections)

	return proj, nil
}

func parseProjectSection(proj *Project, sections map[string]map[string]string) {
	s, ok := sections["PROJECT"]
	if !ok {
		return
	}

	proj.Title = s["TITLE"]
	proj.Version, _ = strconv.Atoi(s["VERSION"])
	proj.V64 = s["V64"] == "1"
}

func parseTimeSlices(proj *Project, sections map[string]map[string]string) {
	s, ok := sections["TIME SLICES DEN"]
	if !ok {
		return
	}

	proj.DayPeriod = s["DAYIDENT"]
	proj.EveningPeriod = s["EVENINGIDENT"]
	proj.NightPeriod = s["NIGHTIDENT"]

	for i := 1; i <= 4; i++ {
		name := s[fmt.Sprintf("ZBNAME%d", i)]
		if name == "" {
			continue
		}

		typeVal := s[fmt.Sprintf("ZBTYPE%d", i)]
		if typeVal != "1" {
			continue
		}

		iso, _ := strconv.Atoi(s[fmt.Sprintf("ZBISO%d", i)])

		proj.AssessmentPeriods = append(proj.AssessmentPeriods, AssessmentPeriod{
			Name:     name,
			ISOLevel: iso,
		})
	}
}

func parseEnabledStandards(proj *Project, sections map[string]map[string]string) {
	s, ok := sections["ENABLEDSTANDARDS"]
	if !ok {
		return
	}

	for key, val := range s {
		id, err := strconv.Atoi(key)
		if err != nil {
			continue
		}

		proj.EnabledStandards[id] = val == "1"
	}
}

func parseSimpleSettings(proj *Project, sections map[string]map[string]string) {
	s, ok := sections["SIMPLESETTINGS"]
	if !ok {
		return
	}

	proj.Settings.ReceiverHeightAboveGround = parseGermanFloat(s["HABOVEGH"])
	proj.Settings.ReceiverHeightAboveFloor = parseGermanFloat(s["HABOVEEFH"])
	proj.Settings.FloorHeight = parseGermanFloat(s["FLOORHEIGHT"])
	proj.Settings.FloorCount, _ = strconv.Atoi(s["FLOORCOUNT"])
	proj.Settings.ReflectionOrder, _ = strconv.Atoi(s["REFLORDNUNG"])
	proj.Settings.RailBonus = s["RAILBONUS"] == "1"
	proj.Settings.GridMapHeight = parseGermanFloat(s["RLKHEIGHT"])
	proj.Settings.GridMapDistance = parseGermanFloat(s["RLKDISTANCE"])
}

func parseGeoDB(proj *Project, sections map[string]map[string]string) {
	s, ok := sections["GEODB"]
	if !ok {
		return
	}

	proj.GeoDB.RelHeightEFH = parseGermanFloat(s["RELHEIGHTEFH"])
	proj.GeoDB.FloorHeight = parseGermanFloat(s["FLOORHEIGHT"])
	proj.GeoDB.ReceiverFacadeDistance = parseGermanFloat(s["RECFACDIST"])
}

func parseNoiseTypeSection(sections map[string]map[string]string, name string) NoiseTypeParams {
	s, ok := sections[name]
	if !ok {
		return NoiseTypeParams{}
	}

	sel, _ := strconv.Atoi(s["SELECTED"])
	paramStr := s[strconv.Itoa(sel)]

	return NoiseTypeParams{
		SelectedStandard: sel,
		ParamString:      paramStr,
	}
}

// RunResult holds the parsed contents of a SoundPlan .res file.
type RunResult struct {
	RKVersion       string
	ProductVersion  string
	ResultSubFolder string
	RunType         string
	RunStart        string
	RunStop         string
	RunCommands     string
	// RunData is the raw `RunData=` line: the input files the kernel was
	// handed, each quoted, the whole list quoted again. RunDataFiles is that
	// line split into the individual names.
	//
	// It duplicates the `[GeoFiles]` section in every .res file seen so far,
	// but the two are written by different parts of SoundPLAN and either can
	// be absent, so GeometryFileNames unions them rather than trusting one.
	RunData      string
	RunDataFiles []string
	ThreadCount  int
	ErrorCode    int
	SourceTypes  int

	Warnings []string

	Statistics RunStatistics
	GeoFiles   []GeoFileRef

	AssessmentPeriods []ResAssessmentPeriod
}

// RunStatistics holds performance/count data from a calculation run.
type RunStatistics struct {
	CalcTimeMS       int
	PointsTotal      int
	PointsCalculated int
}

// GeoFileRef is a reference to a geometry file used in a calculation run.
type GeoFileRef struct {
	Name string
	Date int64
}

// ResAssessmentPeriod holds assessment period info from a .res file.
type ResAssessmentPeriod struct {
	Name        string
	AssessType  string
	AssessHours float64
	Hours       [24]bool
}

// ParseResFile reads and parses a SoundPlan .res result metadata file.
func ParseResFile(path string) (*RunResult, error) {
	sections, err := parseINIFile(path)
	if err != nil {
		return nil, fmt.Errorf("soundplan: parse res: %w", err)
	}

	res := &RunResult{}

	parseResGeneral(res, sections)
	parseResComments(res, sections)
	parseResStatistics(res, sections)
	parseResGeoFiles(res, sections)
	parseResAssessment(res, sections)

	return res, nil
}

func parseResGeneral(res *RunResult, sections map[string]map[string]string) {
	s, ok := sections["General"]
	if !ok {
		return
	}

	res.RKVersion = s["RKVersion"]
	res.ProductVersion = s["Productversion"]
	res.ResultSubFolder = s["ResultSubFolder"]
	res.RunType = s["RunType"]
	res.RunStart = s["RunStart"]
	res.RunStop = s["RunStop"]
	res.RunCommands = s["RunCommands"]
	res.RunData = s["RunData"]
	res.RunDataFiles = parseRunDataFiles(res.RunData)
	res.ThreadCount, _ = strconv.Atoi(s["ThreadCount"])
	res.ErrorCode, _ = strconv.Atoi(s["Error"])
	res.SourceTypes, _ = strconv.Atoi(s["SourceTypes"])
}

// parseRunDataFiles splits a `RunData=` value into the file names it lists.
//
// SoundPLAN writes the list as one doubled-quote string: every name is quoted,
// and the whole sequence is quoted again, e.g.
//
//	RunData=""GeoObjs.geo" "GeoRail.geo" "rdgm0001.dgm""
//
// Splitting on the quote character therefore yields the names interleaved with
// the separators and the empty strings the doubled outer quotes produce, so
// everything that is blank after trimming is dropped. A bare, unquoted value is
// returned as a single name.
func parseRunDataFiles(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	fields := strings.Split(raw, `"`)
	names := make([]string, 0, len(fields)/2+1)

	for _, field := range fields {
		name := strings.TrimSpace(field)
		if name == "" {
			continue
		}

		names = append(names, name)
	}

	if len(names) == 0 {
		return nil
	}

	return names
}

// GeometryFileNames reports the input files this run consumed, as lower-cased
// base names, sorted and deduplicated.
//
// It unions `[GeoFiles]` with `RunData`, which is what makes it usable for
// telling two otherwise identical runs apart: in the reference project
// RSPS0011 and RSPS0021 compute the same 13 immission points and differ only
// in whether `GeoWand.geo` — the noise barrier — was part of the input.
func (res *RunResult) GeometryFileNames() []string {
	if res == nil {
		return nil
	}

	seen := make(map[string]struct{}, len(res.GeoFiles)+len(res.RunDataFiles))
	names := make([]string, 0, len(res.GeoFiles)+len(res.RunDataFiles))

	add := func(raw string) {
		name := strings.ToLower(strings.TrimSpace(raw))
		name = strings.ReplaceAll(name, `\`, "/")

		if idx := strings.LastIndexByte(name, '/'); idx >= 0 {
			name = name[idx+1:]
		}

		if name == "" {
			return
		}

		if _, ok := seen[name]; ok {
			return
		}

		seen[name] = struct{}{}

		names = append(names, name)
	}

	for _, ref := range res.GeoFiles {
		add(ref.Name)
	}

	for _, name := range res.RunDataFiles {
		add(name)
	}

	slices.Sort(names)

	return names
}

// gridMapRunCommandPrefix is the token SoundPLAN writes into `RunCommands` for
// a grid-map run: `GNM<spacing>:<height>`, e.g. `GNM5:4`.
const gridMapRunCommandPrefix = "GNM"

// GridMapLayout reports the grid spacing and receiver height this run was
// computed with, read out of the `GNM<spacing>:<height>` token in
// `RunCommands`. The second return is false when no such token is present or
// it cannot be read, which must not be confused with a run computed at ground
// level with zero spacing.
//
// It is the second discriminator between otherwise identical grid-map runs. In
// the reference project the geometry list separates the four RRLK runs into
// two computed without the noise barrier and two with it, and only the height
// in this token tells the remaining pair apart.
func (res *RunResult) GridMapLayout() (GridMapRunLayout, bool) {
	if res == nil {
		return GridMapRunLayout{}, false
	}

	index := strings.Index(strings.ToUpper(res.RunCommands), gridMapRunCommandPrefix)
	if index < 0 {
		return GridMapRunLayout{}, false
	}

	// The token is read as the maximal numeric run after the prefix rather than
	// by splitting the line on a separator: SoundPLAN writes German decimals,
	// so a comma is as likely to sit inside a number as between two commands.
	token := res.RunCommands[index+len(gridMapRunCommandPrefix):]
	if end := strings.IndexFunc(token, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.' && r != ',' && r != ':'
	}); end >= 0 {
		token = token[:end]
	}

	spacing, height, ok := strings.Cut(token, ":")
	if !ok {
		return GridMapRunLayout{}, false
	}

	spacingM, spacingOK := parseDecimal(spacing)

	heightM, heightOK := parseDecimal(height)
	if !spacingOK || !heightOK || spacingM <= 0 {
		return GridMapRunLayout{}, false
	}

	return GridMapRunLayout{SpacingM: spacingM, HeightM: heightM}, true
}

// parseDecimal reads a SoundPLAN number that may use either decimal separator.
// Unlike parseGermanFloat it says whether it succeeded, because a caller that
// has to tell "absent" from "zero" cannot use a zero return to do it.
func parseDecimal(raw string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(raw), ",", "."), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}

	return value, true
}

func parseResComments(res *RunResult, sections map[string]map[string]string) {
	s, ok := sections["Comments"]
	if !ok {
		return
	}

	nWarnings, _ := strconv.Atoi(s["Warnings"])

	for i := 1; i <= nWarnings; i++ {
		w := s[fmt.Sprintf("Warning%d", i)]
		if w != "" {
			res.Warnings = append(res.Warnings, w)
		}
	}
}

func parseResStatistics(res *RunResult, sections map[string]map[string]string) {
	s, ok := sections["Statistics"]
	if !ok {
		return
	}

	res.Statistics.CalcTimeMS, _ = strconv.Atoi(s["CalcTime"])
	res.Statistics.PointsTotal, _ = strconv.Atoi(s["NoPointsTotal"])
	res.Statistics.PointsCalculated, _ = strconv.Atoi(s["NoPointsCalculated"])
}

func parseResGeoFiles(res *RunResult, sections map[string]map[string]string) {
	s, ok := sections["GeoFiles"]
	if !ok {
		return
	}

	for i := 0; ; i++ {
		nameKey := fmt.Sprintf("FileName%d", i)
		dateKey := fmt.Sprintf("FileDate%d", i)

		name, nameOK := s[nameKey]
		if !nameOK {
			break
		}

		date, _ := strconv.ParseInt(s[dateKey], 10, 64)
		res.GeoFiles = append(res.GeoFiles, GeoFileRef{Name: name, Date: date})
	}
}

func parseResAssessment(res *RunResult, sections map[string]map[string]string) {
	for i := 1; i <= 4; i++ {
		sectionName := fmt.Sprintf("Assessment.ZB%d", i)

		s, ok := sections[sectionName]
		if !ok {
			continue
		}

		name := s["ZBName"]
		if name == "" {
			continue
		}

		hours := parseHourMask(s["Hours"])
		assessHours, _ := strconv.ParseFloat(s["TAssess"], 64)

		res.AssessmentPeriods = append(res.AssessmentPeriods, ResAssessmentPeriod{
			Name:        name,
			AssessType:  s["AssessType"],
			AssessHours: assessHours,
			Hours:       hours,
		})
	}
}

// ListRuns discovers all .res files in a SoundPlan project directory
// and parses their metadata.
func ListRuns(projectDir string) ([]*RunResult, error) {
	matches, err := filepath.Glob(filepath.Join(projectDir, "*.res"))
	if err != nil {
		return nil, fmt.Errorf("soundplan: glob res files: %w", err)
	}

	var runs []*RunResult

	for _, p := range matches {
		res, parseErr := ParseResFile(p)
		if parseErr != nil {
			return nil, fmt.Errorf("soundplan: parse %s: %w", filepath.Base(p), parseErr)
		}

		runs = append(runs, res)
	}

	return runs, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// parseINIFile reads a Windows-1252 INI file into section→key→value maps.
// Section names are case-sensitive to match SoundPlan conventions.
func parseINIFile(path string) (map[string]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open ini file %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	reader := transform.NewReader(f, charmap.Windows1252.NewDecoder())
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	sections := make(map[string]map[string]string)
	currentSection := ""

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")

		if len(line) == 0 {
			continue
		}

		// Section header.
		if line[0] == '[' {
			end := strings.IndexByte(line, ']')
			if end > 0 {
				currentSection = line[1:end]

				if _, ok := sections[currentSection]; !ok {
					sections[currentSection] = make(map[string]string)
				}
			}

			continue
		}

		// Key=Value pair.
		key, val, found := strings.Cut(line, "=")
		if !found || currentSection == "" {
			continue
		}

		sections[currentSection][key] = val
	}

	scanErr := scanner.Err()
	if scanErr != nil {
		return nil, fmt.Errorf("scan: %w", scanErr)
	}

	return sections, nil
}

// parseGermanFloat parses a float that uses German comma as decimal separator.
func parseGermanFloat(s string) float64 {
	s = strings.ReplaceAll(s, ",", ".")
	v, _ := strconv.ParseFloat(s, 64)

	return v
}

// parseHourMask parses a comma-separated list of 24 "0"/"1" values
// into a boolean array indexed by hour (0-23).
func parseHourMask(s string) [24]bool {
	var hours [24]bool

	parts := strings.Split(s, ",")

	for i, p := range parts {
		if i >= 24 {
			break
		}

		hours[i] = strings.TrimSpace(p) == "1" //nolint:gosec // i < 24 guarded by break above
	}

	return hours
}
