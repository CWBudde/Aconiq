// Package soundplanimport reads SoundPlan project files (.sp) and associated
// result metadata (.res) into structured Go types for cross-validation.
package soundplanimport

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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
	ThreadCount     int
	ErrorCode       int
	SourceTypes     int

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
	res.ThreadCount, _ = strconv.Atoi(s["ThreadCount"])
	res.ErrorCode, _ = strconv.Atoi(s["Error"])
	res.SourceTypes, _ = strconv.Atoi(s["SourceTypes"])
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
		return nil, err
	}
	defer f.Close()

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
