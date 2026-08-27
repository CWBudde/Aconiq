package soundplanimport

import (
	"errors"
	"math"
	"path/filepath"
	"strings"

	"github.com/aconiq/backend/internal/standards/schall03"
)

// RailOperationSummary captures derived per-track operating inputs from
// SoundPLAN rail emission and train-emission result tables.
type RailOperationSummary struct {
	ObjID              int32
	Railname           string
	TrainClass         string
	TractionType       string
	AverageSpeedKPH    float64
	TrafficDayPH       float64
	TrafficNightPH     float64
	OnBridge           bool
	DominantTrainName  string
	TrainNames         []string
	DayTrainCount      float64
	NightTrainCount    float64
	TrackVMaxKPH       float64
	AssessmentDayHours float64
	AssessmentNightHrs float64
}

// LoadRailOperationSummaries derives rail operating inputs from one preferred
// SoundPLAN run directory containing both RRAI and RRAD tables.
func LoadRailOperationSummaries(projectDir string, proj *Project, runs []*RunResult) ([]RailOperationSummary, string, error) {
	resultDir, err := selectRailOperationResultDir(projectDir, runs)
	if err != nil {
		return nil, "", err
	}

	base := filepath.Base(resultDir)
	suffix := extractRunSuffix(base)

	railEmissions, err := ParseRailEmissions(filepath.Join(resultDir, "RRAI"+suffix+".abs"))
	if err != nil {
		return nil, "", err
	}

	trainEmissions, err := ParseTrainEmissions(filepath.Join(resultDir, "RRAD"+suffix+".abs"))
	if err != nil {
		return nil, "", err
	}

	trainTypes, _ := ParseTrainTypes(filepath.Join(projectDir, "TS03.abs"))
	typeByName := buildTrainTypeIndex(trainTypes)

	dayHours, nightHours := deriveAssessmentHours(proj)
	trainsByIDX := groupTrainEmissionsByIDX(trainEmissions)

	summaries := make([]RailOperationSummary, 0, len(railEmissions))
	for _, rail := range railEmissions {
		summaries = append(summaries, buildRailOperationSummary(rail, trainsByIDX[rail.IDX], typeByName, dayHours, nightHours))
	}

	return summaries, resultDir, nil
}

// buildTrainTypeIndex indexes train types by their trimmed name for lookup
// during rail operation summary derivation.
func buildTrainTypeIndex(trainTypes []TrainType) map[string]TrainType {
	typeByName := make(map[string]TrainType, len(trainTypes))
	for _, trainType := range trainTypes {
		if name := strings.TrimSpace(trainType.Name); name != "" {
			typeByName[name] = trainType
		}
	}

	return typeByName
}

// groupTrainEmissionsByIDX groups train emission rows by their rail track
// IDX reference.
func groupTrainEmissionsByIDX(trainEmissions []TrainEmission) map[int32][]TrainEmission {
	trainsByIDX := make(map[int32][]TrainEmission)
	for _, emission := range trainEmissions {
		trainsByIDX[emission.IDX] = append(trainsByIDX[emission.IDX], emission)
	}

	return trainsByIDX
}

// buildRailOperationSummary derives one rail track's operation summary from
// its emission row and the train emissions linked to it.
func buildRailOperationSummary(rail RailEmission, linked []TrainEmission, typeByName map[string]TrainType, dayHours, nightHours float64) RailOperationSummary {
	summary := RailOperationSummary{
		ObjID:              rail.ObjID,
		Railname:           strings.TrimSpace(rail.Railname),
		TrackVMaxKPH:       rail.TrackV,
		OnBridge:           rail.DBue > 0,
		AssessmentDayHours: dayHours,
		AssessmentNightHrs: nightHours,
	}

	if len(linked) == 0 {
		if rail.TrackV > 0 {
			summary.AverageSpeedKPH = rail.TrackV
		}

		summary.TrainClass = schall03.TrainClassMixed

		return summary
	}

	totalWeight := 0.0
	dominantWeight := -1.0
	classSeen := make(map[string]struct{})
	tractionSeen := make(map[string]struct{})
	trainNames := make([]string, 0, len(linked))
	// Membership set for trainNames: a linear scan per train makes this loop
	// quadratic in the number of trains the imported file declares.
	nameSeen := make(map[string]struct{}, len(linked))

	for _, train := range linked {
		summary.DayTrainCount += train.NDay
		summary.NightTrainCount += train.NNight

		weight := train.NDay + train.NNight
		if weight > 0 && train.Speed > 0 {
			summary.AverageSpeedKPH += train.Speed * weight
			totalWeight += weight
		}

		name := strings.TrimSpace(train.Trainname)
		if _, dup := nameSeen[name]; name != "" && !dup {
			nameSeen[name] = struct{}{}

			trainNames = append(trainNames, name)
		}

		if weight > dominantWeight && name != "" {
			dominantWeight = weight
			summary.DominantTrainName = name
		}

		trainType := typeByName[name]
		classSeen[classifyTrainClass(name, trainType)] = struct{}{}
		tractionSeen[classifyTractionType(name, trainType)] = struct{}{}
	}

	if totalWeight > 0 {
		summary.AverageSpeedKPH /= totalWeight
	} else if rail.TrackV > 0 {
		summary.AverageSpeedKPH = rail.TrackV
	}

	if dayHours > 0 {
		summary.TrafficDayPH = summary.DayTrainCount / dayHours
	}

	if nightHours > 0 {
		summary.TrafficNightPH = summary.NightTrainCount / nightHours
	}

	summary.TrainNames = trainNames
	summary.TrainClass = collapseTrainClasses(classSeen)
	summary.TractionType = collapseTractionTypes(tractionSeen)

	return summary
}

func selectRailOperationResultDir(projectDir string, runs []*RunResult) (string, error) {
	preferredKinds := []string{"RSPS", "RRLK"}

	for _, prefix := range preferredKinds {
		for _, run := range runs {
			if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(run.ResultSubFolder)), prefix) {
				continue
			}

			dir := filepath.Join(projectDir, run.ResultSubFolder)

			suffix := extractRunSuffix(run.ResultSubFolder)
			if fileExists(filepath.Join(dir, "RRAI"+suffix+".abs")) && fileExists(filepath.Join(dir, "RRAD"+suffix+".abs")) {
				return dir, nil
			}
		}
	}

	return "", errors.New("soundplan: no result subdirectory with both RRAI and RRAD tables found")
}

func deriveAssessmentHours(proj *Project) (float64, float64) {
	if proj == nil {
		return 16, 8
	}

	dayHours := durationHours(proj.DayPeriod)
	nightHours := durationHours(proj.NightPeriod)

	if dayHours <= 0 {
		dayHours = 16
	}

	if nightHours <= 0 {
		nightHours = 8
	}

	return dayHours, nightHours
}

func durationHours(period string) float64 {
	parts := strings.Split(strings.TrimSpace(period), "-")
	if len(parts) != 2 {
		return 0
	}

	start := parseGermanFloat(parts[0])
	end := parseGermanFloat(parts[1])

	duration := end - start
	if duration <= 0 {
		duration += 24
	}

	if duration <= 0 || duration > 24 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return 0
	}

	return duration
}

func classifyTrainClass(name string, trainType TrainType) string {
	switch trainType.ZugArt {
	case 6, 7:
		return schall03.TrainClassFreight
	case 1, 2, 3, 4, 5, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18:
		return schall03.TrainClassPassenger
	}

	lower := strings.ToLower(strings.TrimSpace(name))

	switch {
	case strings.Contains(lower, "güter"):
		return schall03.TrainClassFreight
	case strings.Contains(lower, "straßenbahn"), strings.Contains(lower, "strassenbahn"), strings.Contains(lower, "s-bahn"), strings.Contains(lower, "u - bahn"), strings.Contains(lower, "u-bahn"), strings.Contains(lower, "ice"), strings.Contains(lower, "ec"), strings.Contains(lower, "ic"), strings.Contains(lower, "eilzug"), strings.Contains(lower, "nahverkehr"), strings.Contains(lower, "inter regio"), strings.Contains(lower, "d / fd-zug"):
		return schall03.TrainClassPassenger
	}

	typeName := strings.ToLower(strings.TrimSpace(trainType.Name))
	if strings.Contains(typeName, "güter") {
		return schall03.TrainClassFreight
	}

	return schall03.TrainClassMixed
}

func classifyTractionType(name string, trainType TrainType) string {
	switch trainType.ZugArt {
	case 8, 9, 13, 14, 15, 16, 17, 18:
		return schall03.TractionElectric
	}

	lower := strings.ToLower(strings.TrimSpace(name))
	typeName := strings.ToLower(strings.TrimSpace(trainType.Name))

	switch {
	case strings.Contains(lower, "e-lok"), strings.Contains(typeName, "e-lok"):
		return schall03.TractionElectric
	case strings.Contains(lower, "v-lok"), strings.Contains(typeName, "v-lok"), strings.Contains(lower, "diesel"), strings.Contains(typeName, "diesel"):
		return schall03.TractionDiesel
	case strings.Contains(lower, "straßenbahn"), strings.Contains(lower, "strassenbahn"), strings.Contains(lower, "s-bahn"), strings.Contains(lower, "u - bahn"), strings.Contains(lower, "u-bahn"), strings.Contains(lower, "ice"),
		strings.Contains(typeName, "straßenbahn"), strings.Contains(typeName, "strassenbahn"), strings.Contains(typeName, "s-bahn"), strings.Contains(typeName, "u - bahn"), strings.Contains(typeName, "u-bahn"), strings.Contains(typeName, "ice"):
		return schall03.TractionElectric
	default:
		return schall03.TractionMixed
	}
}

func collapseTrainClasses(seen map[string]struct{}) string {
	if len(seen) == 0 {
		return schall03.TrainClassMixed
	}

	if len(seen) == 1 {
		for class := range seen {
			return class
		}
	}

	return schall03.TrainClassMixed
}

func collapseTractionTypes(seen map[string]struct{}) string {
	if len(seen) == 0 {
		return schall03.TractionMixed
	}

	if len(seen) == 1 {
		for traction := range seen {
			return traction
		}
	}

	return schall03.TractionMixed
}
