package bimschv16

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/report/results"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
	"github.com/aconiq/backend/internal/standards/schall03"
)

const LawName = "16. BImSchV"

type AreaCategory string

const (
	AreaHospitalsSchoolsCareHomes AreaCategory = "hospital_school_kurheim_altenheim"
	AreaResidential               AreaCategory = "residential"
	AreaMixed                     AreaCategory = "mixed"
	AreaCommercial                AreaCategory = "commercial"
)

type Thresholds struct {
	Day   int `json:"day"`
	Night int `json:"night"`
}

type PeriodLevels struct {
	Day   float64 `json:"day"`
	Night float64 `json:"night"`
}

type LevelAssessment struct {
	DayRaw            float64 `json:"day_raw"`
	NightRaw          float64 `json:"night_raw"`
	DayRounded        int     `json:"day_rounded"`
	NightRounded      int     `json:"night_rounded"`
	DayExceeds        bool    `json:"day_exceeds"`
	NightExceeds      bool    `json:"night_exceeds"`
	DayExceedanceDB   int     `json:"day_exceedance_db"`
	NightExceedanceDB int     `json:"night_exceedance_db"`
}

type ReceiverInput struct {
	ReceiverID   string
	AreaCategory AreaCategory
	Road         *PeriodLevels
	Rail         *PeriodLevels
}

type ReceiverAssessment struct {
	ReceiverID                         string           `json:"receiver_id"`
	AreaCategory                       AreaCategory     `json:"area_category"`
	AreaCategoryLabelDE                string           `json:"area_category_label_de"`
	Thresholds                         Thresholds       `json:"thresholds"`
	Road                               *LevelAssessment `json:"road,omitempty"`
	Rail                               *LevelAssessment `json:"rail,omitempty"`
	Combined                           *LevelAssessment `json:"combined,omitempty"`
	EligibleForNoiseProtectionMeasures bool             `json:"eligible_for_noise_protection_measures"`
	SummaryDE                          string           `json:"summary_de"`
}

type SkippedReceiver struct {
	ReceiverID string `json:"receiver_id"`
	Reason     string `json:"reason"`
}

type ExportEnvelope struct {
	Law              string               `json:"law"`
	GeneratedAt      time.Time            `json:"generated_at"`
	SourceStandardID string               `json:"source_standard_id"`
	ReceiverCount    int                  `json:"receiver_count"`
	AssessedCount    int                  `json:"assessed_count"`
	ExceedingCount   int                  `json:"exceeding_count"`
	CategoryCounts   map[string]int       `json:"category_counts,omitempty"`
	Results          []ReceiverAssessment `json:"results"`
	Skipped          []SkippedReceiver    `json:"skipped,omitempty"`
}

func ThresholdsForCategory(category AreaCategory) (Thresholds, error) {
	switch category {
	case AreaHospitalsSchoolsCareHomes:
		return Thresholds{Day: 57, Night: 47}, nil
	case AreaResidential:
		return Thresholds{Day: 59, Night: 49}, nil
	case AreaMixed:
		return Thresholds{Day: 64, Night: 54}, nil
	case AreaCommercial:
		return Thresholds{Day: 69, Night: 59}, nil
	default:
		return Thresholds{}, fmt.Errorf("unsupported 16. BImSchV area category %q", category)
	}
}

func AreaCategoryLabelDE(category AreaCategory) string {
	switch category {
	case AreaHospitalsSchoolsCareHomes:
		return "Krankenhaus, Schule, Kurheim oder Altenheim"
	case AreaResidential:
		return "Wohngebiet oder Kleinsiedlungsgebiet"
	case AreaMixed:
		return "Kern-, Dorf- oder Mischgebiet"
	case AreaCommercial:
		return "Gewerbegebiet"
	default:
		return string(category)
	}
}

func ParseAreaCategory(raw string) (AreaCategory, error) {
	switch normalizeCategory(raw) {
	case "hospitalschoolkurheimaltenheim", "hospitalschoolkurheimcarehome", "krankenhaus", "schule", "kurheim", "altenheim":
		return AreaHospitalsSchoolsCareHomes, nil
	case "residential", "wohngebiet", "reineswohngebiet", "allgemeineswohngebiet", "kleinsiedlungsgebiet":
		return AreaResidential, nil
	case "mixed", "mischgebiet", "kerngebiet", "dorfgebiet":
		return AreaMixed, nil
	case "commercial", "gewerbegebiet":
		return AreaCommercial, nil
	default:
		return "", fmt.Errorf("unknown 16. BImSchV area category %q", raw)
	}
}

func normalizeCategory(raw string) string {
	text := strings.ToLower(strings.TrimSpace(raw))
	replacer := strings.NewReplacer(
		"ä", "ae",
		"ö", "oe",
		"ü", "ue",
		"ß", "ss",
		"-", "",
		"_", "",
		"/", "",
		",", "",
		" ", "",
	)

	return replacer.Replace(text)
}

func RoundForThresholdComparison(level float64) int {
	return int(math.Ceil(level))
}

func assessLevels(levels PeriodLevels, thresholds Thresholds) (*LevelAssessment, error) {
	if math.IsNaN(levels.Day) || math.IsNaN(levels.Night) || math.IsInf(levels.Day, 0) || math.IsInf(levels.Night, 0) {
		return nil, errors.New("levels must be finite")
	}

	dayRounded := RoundForThresholdComparison(levels.Day)
	nightRounded := RoundForThresholdComparison(levels.Night)

	dayExceedance := maxInt(0, dayRounded-thresholds.Day)
	nightExceedance := maxInt(0, nightRounded-thresholds.Night)

	return &LevelAssessment{
		DayRaw:            levels.Day,
		NightRaw:          levels.Night,
		DayRounded:        dayRounded,
		NightRounded:      nightRounded,
		DayExceeds:        dayExceedance > 0,
		NightExceeds:      nightExceedance > 0,
		DayExceedanceDB:   dayExceedance,
		NightExceedanceDB: nightExceedance,
	}, nil
}

func combineLevels(inputs ...*PeriodLevels) *PeriodLevels {
	dayTerms := make([]float64, 0, len(inputs))

	nightTerms := make([]float64, 0, len(inputs))
	for _, input := range inputs {
		if input == nil {
			continue
		}

		dayTerms = append(dayTerms, input.Day)
		nightTerms = append(nightTerms, input.Night)
	}

	if len(dayTerms) == 0 {
		return nil
	}

	return &PeriodLevels{
		Day:   energySumDB(dayTerms),
		Night: energySumDB(nightTerms),
	}
}

func energySumDB(levels []float64) float64 {
	sum := 0.0
	for _, level := range levels {
		sum += math.Pow(10, level/10)
	}

	return 10 * math.Log10(sum)
}

func AssessReceiver(input ReceiverInput) (ReceiverAssessment, error) {
	if strings.TrimSpace(input.ReceiverID) == "" {
		return ReceiverAssessment{}, errors.New("receiver id is required")
	}

	if input.Road == nil && input.Rail == nil {
		return ReceiverAssessment{}, errors.New("at least one road or rail level set is required")
	}

	thresholds, err := ThresholdsForCategory(input.AreaCategory)
	if err != nil {
		return ReceiverAssessment{}, err
	}

	result := ReceiverAssessment{
		ReceiverID:          input.ReceiverID,
		AreaCategory:        input.AreaCategory,
		AreaCategoryLabelDE: AreaCategoryLabelDE(input.AreaCategory),
		Thresholds:          thresholds,
	}

	if input.Road != nil {
		result.Road, err = assessLevels(*input.Road, thresholds)
		if err != nil {
			return ReceiverAssessment{}, fmt.Errorf("assess road levels: %w", err)
		}
	}

	if input.Rail != nil {
		result.Rail, err = assessLevels(*input.Rail, thresholds)
		if err != nil {
			return ReceiverAssessment{}, fmt.Errorf("assess rail levels: %w", err)
		}
	}

	combined := combineLevels(input.Road, input.Rail)
	if combined != nil {
		result.Combined, err = assessLevels(*combined, thresholds)
		if err != nil {
			return ReceiverAssessment{}, fmt.Errorf("assess combined levels: %w", err)
		}
	}

	result.EligibleForNoiseProtectionMeasures = result.exceedsAnyThreshold()
	result.SummaryDE = buildSummaryDE(result)

	return result, nil
}

func (r ReceiverAssessment) exceedsAnyThreshold() bool {
	for _, check := range []*LevelAssessment{r.Combined, r.Road, r.Rail} {
		if check == nil {
			continue
		}

		if check.DayExceeds || check.NightExceeds {
			return true
		}
	}

	return false
}

func buildSummaryDE(result ReceiverAssessment) string {
	base := fmt.Sprintf(
		"Empfänger %s in Kategorie %s: maßgebliche Immissionsgrenzwerte Tag/Nacht %d/%d dB.",
		result.ReceiverID,
		result.AreaCategoryLabelDE,
		result.Thresholds.Day,
		result.Thresholds.Night,
	)

	label := "maßgeblicher Beurteilungspegel"

	assessment := result.Combined
	switch {
	case result.Road != nil && result.Rail == nil:
		label = "Beurteilungspegel Straße"
		assessment = result.Road
	case result.Rail != nil && result.Road == nil:
		label = "Beurteilungspegel Schiene"
		assessment = result.Rail
	case result.Road != nil && result.Rail != nil:
		label = "kombinierter Beurteilungspegel Straße + Schiene"
	}

	if assessment == nil {
		return base
	}

	status := "Die Immissionsgrenzwerte werden eingehalten."

	if assessment.DayExceeds || assessment.NightExceeds {
		switch {
		case assessment.DayExceeds && assessment.NightExceeds:
			status = fmt.Sprintf("Die Immissionsgrenzwerte werden tags um %d dB und nachts um %d dB überschritten. Ein Anspruch auf Lärmschutzmaßnahmen ist dem Grunde nach gegeben.", assessment.DayExceedanceDB, assessment.NightExceedanceDB)
		case assessment.DayExceeds:
			status = fmt.Sprintf("Die Immissionsgrenzwerte werden tags um %d dB überschritten. Ein Anspruch auf Lärmschutzmaßnahmen ist dem Grunde nach gegeben.", assessment.DayExceedanceDB)
		default:
			status = fmt.Sprintf("Die Immissionsgrenzwerte werden nachts um %d dB überschritten. Ein Anspruch auf Lärmschutzmaßnahmen ist dem Grunde nach gegeben.", assessment.NightExceedanceDB)
		}
	}

	return fmt.Sprintf("%s %s %d/%d dB. %s", base, label, assessment.DayRounded, assessment.NightRounded, status)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func assessmentLevelsFromRecord(record results.ReceiverRecord) (PeriodLevels, error) {
	day, ok := record.Values[rls19road.IndicatorLrDay]
	if !ok {
		day, ok = record.Values[schall03.IndicatorLrDay]
	}

	if !ok {
		return PeriodLevels{}, errors.New("receiver table record missing LrDay")
	}

	night, ok := record.Values[rls19road.IndicatorLrNight]
	if !ok {
		night, ok = record.Values[schall03.IndicatorLrNight]
	}

	if !ok {
		return PeriodLevels{}, errors.New("receiver table record missing LrNight")
	}

	return PeriodLevels{Day: day, Night: night}, nil
}

func categoryFromFeature(feature modelgeojson.Feature) (AreaCategory, bool, error) {
	for _, key := range []string{
		"bimschv16_area_category",
		"bimschv16_gebietskategorie",
		"area_category",
		"gebietskategorie",
		"immissionsgrenzwert_kategorie",
	} {
		raw, ok := feature.Properties[key]
		if !ok || raw == nil {
			continue
		}

		text, ok := raw.(string)
		if !ok {
			return "", false, fmt.Errorf("property %q must be a string", key)
		}

		category, err := ParseAreaCategory(text)
		if err != nil {
			return "", false, err
		}

		return category, true, nil
	}

	return "", false, nil
}

func supportedSourceStandard(standardID string) bool {
	switch standardID {
	case rls19road.StandardID, schall03.StandardID:
		return true
	default:
		return false
	}
}

func BuildExportEnvelope(model modelgeojson.Model, table results.ReceiverTable, sourceStandardID string, generatedAt time.Time) (ExportEnvelope, error) {
	if !supportedSourceStandard(sourceStandardID) {
		return ExportEnvelope{}, fmt.Errorf("16. BImSchV assessment currently supports only %q and %q, got %q", rls19road.StandardID, schall03.StandardID, sourceStandardID)
	}

	recordsByID := make(map[string]results.ReceiverRecord, len(table.Records))
	for _, record := range table.Records {
		recordsByID[record.ID] = record
	}

	out := ExportEnvelope{
		Law:              LawName,
		GeneratedAt:      generatedAt.UTC(),
		SourceStandardID: sourceStandardID,
		Results:          make([]ReceiverAssessment, 0),
		Skipped:          make([]SkippedReceiver, 0),
		CategoryCounts:   make(map[string]int),
	}

	for _, feature := range model.Features {
		if feature.Kind != "receiver" {
			continue
		}

		out.ReceiverCount++

		category, ok, err := categoryFromFeature(feature)
		if err != nil {
			out.Skipped = append(out.Skipped, SkippedReceiver{ReceiverID: feature.ID, Reason: err.Error()})
			continue
		}

		if !ok {
			out.Skipped = append(out.Skipped, SkippedReceiver{ReceiverID: feature.ID, Reason: "missing 16. BImSchV area category property"})
			continue
		}

		record, ok := recordsByID[feature.ID]
		if !ok {
			out.Skipped = append(out.Skipped, SkippedReceiver{ReceiverID: feature.ID, Reason: "receiver not found in result table"})
			continue
		}

		levels, err := assessmentLevelsFromRecord(record)
		if err != nil {
			out.Skipped = append(out.Skipped, SkippedReceiver{ReceiverID: feature.ID, Reason: err.Error()})
			continue
		}

		input := ReceiverInput{
			ReceiverID:   feature.ID,
			AreaCategory: category,
		}

		switch sourceStandardID {
		case rls19road.StandardID:
			input.Road = &levels
		case schall03.StandardID:
			input.Rail = &levels
		}

		result, err := AssessReceiver(input)
		if err != nil {
			out.Skipped = append(out.Skipped, SkippedReceiver{ReceiverID: feature.ID, Reason: err.Error()})
			continue
		}

		out.AssessedCount++
		if result.EligibleForNoiseProtectionMeasures {
			out.ExceedingCount++
		}

		out.CategoryCounts[string(result.AreaCategory)]++
		out.Results = append(out.Results, result)
	}

	sort.Slice(out.Results, func(i, j int) bool { return out.Results[i].ReceiverID < out.Results[j].ReceiverID })
	sort.Slice(out.Skipped, func(i, j int) bool { return out.Skipped[i].ReceiverID < out.Skipped[j].ReceiverID })

	if len(out.CategoryCounts) == 0 {
		out.CategoryCounts = nil
	}

	return out, nil
}
