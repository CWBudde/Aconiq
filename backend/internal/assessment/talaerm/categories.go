package talaerm

import (
	"fmt"
	"strings"
)

const RegulationName = "TA Lärm"

const RegulationEdition = "26.08.1998 (GMBl. S. 503), geändert 01.06.2017 (BAnz. AT 08.06.2017 B5)"

// AreaCategory identifies an area use category per TA Lärm Nr. 6.1.
type AreaCategory string

const (
	AreaIndustrial      AreaCategory = "industrial"       // a) Industriegebiete
	AreaCommercial      AreaCategory = "commercial"       // b) Gewerbegebiete
	AreaUrban           AreaCategory = "urban"            // c) urbane Gebiete (2017 amendment)
	AreaMixed           AreaCategory = "mixed"            // d) Kern-/Dorf-/Mischgebiete
	AreaResidential     AreaCategory = "residential"      // e) allgemeine Wohn-/Kleinsiedlungsgebiete
	AreaPureResidential AreaCategory = "pure_residential" // f) reine Wohngebiete
	AreaHealthcare      AreaCategory = "healthcare"       // g) Kurgebiete/Krankenhäuser/Pflegeanstalten
)

// Thresholds holds the immission guideline values for day and night periods.
type Thresholds struct {
	Day   int `json:"day"`
	Night int `json:"night"`
}

// PeakLimits holds the maximum permissible peak levels for day and night periods.
type PeakLimits struct {
	Day   int `json:"day"`
	Night int `json:"night"`
}

// ThresholdsOutdoor returns the immission guideline values per TA Lärm Nr. 6.1.
func ThresholdsOutdoor(category AreaCategory) (Thresholds, error) {
	switch category {
	case AreaIndustrial:
		return Thresholds{Day: 70, Night: 70}, nil
	case AreaCommercial:
		return Thresholds{Day: 65, Night: 50}, nil
	case AreaUrban:
		return Thresholds{Day: 63, Night: 45}, nil
	case AreaMixed:
		return Thresholds{Day: 60, Night: 45}, nil
	case AreaResidential:
		return Thresholds{Day: 55, Night: 40}, nil
	case AreaPureResidential:
		return Thresholds{Day: 50, Night: 35}, nil
	case AreaHealthcare:
		return Thresholds{Day: 45, Night: 35}, nil
	default:
		return Thresholds{}, fmt.Errorf("unsupported TA Lärm area category %q", category)
	}
}

// ThresholdsIndoor returns the indoor immission guideline values per TA Lärm Nr. 6.2.
func ThresholdsIndoor() Thresholds {
	return Thresholds{Day: 35, Night: 25}
}

// ThresholdsRareEvents returns the immission guideline values for rare events per TA Lärm Nr. 6.3.
func ThresholdsRareEvents() Thresholds {
	return Thresholds{Day: 70, Night: 55}
}

// PeakLimitsOutdoor returns the maximum permissible peak levels per TA Lärm Nr. 6.1.
// Day: threshold + 30, Night: threshold + 20.
func PeakLimitsOutdoor(category AreaCategory) (PeakLimits, error) {
	th, err := ThresholdsOutdoor(category)
	if err != nil {
		return PeakLimits{}, err
	}

	return PeakLimits{Day: th.Day + 30, Night: th.Night + 20}, nil
}

// PeakLimitsIndoor returns the maximum permissible indoor peak levels per TA Lärm Nr. 6.2.
// Day: 35+10=45, Night: 25+10=35.
func PeakLimitsIndoor() PeakLimits {
	return PeakLimits{Day: 45, Night: 35}
}

// PeakLimitsRareEvents returns the maximum permissible peak levels for rare events per TA Lärm Nr. 6.3.
// Not applicable for industrial areas (category a).
func PeakLimitsRareEvents(category AreaCategory) (PeakLimits, error) {
	switch category {
	case AreaIndustrial:
		return PeakLimits{}, fmt.Errorf("TA Lärm Nr. 6.3 peak limits not applicable for %q (industrial areas)", category)
	case AreaCommercial:
		return PeakLimits{Day: 95, Night: 70}, nil
	case AreaUrban, AreaMixed, AreaResidential, AreaPureResidential, AreaHealthcare:
		return PeakLimits{Day: 90, Night: 65}, nil
	default:
		return PeakLimits{}, fmt.Errorf("unsupported TA Lärm area category %q", category)
	}
}

// AreaCategoryLabelDE returns the German designation for the area category.
func AreaCategoryLabelDE(category AreaCategory) string {
	switch category {
	case AreaIndustrial:
		return "Industriegebiet"
	case AreaCommercial:
		return "Gewerbegebiet"
	case AreaUrban:
		return "urbanes Gebiet"
	case AreaMixed:
		return "Kern-, Dorf- oder Mischgebiet"
	case AreaResidential:
		return "allgemeines Wohngebiet oder Kleinsiedlungsgebiet"
	case AreaPureResidential:
		return "reines Wohngebiet"
	case AreaHealthcare:
		return "Kurgebiet, Krankenhaus oder Pflegeanstalt"
	default:
		return string(category)
	}
}

// AreaCategoryCode returns the letter code ("a" through "g") for the area category.
func AreaCategoryCode(category AreaCategory) string {
	switch category {
	case AreaIndustrial:
		return "a"
	case AreaCommercial:
		return "b"
	case AreaUrban:
		return "c"
	case AreaMixed:
		return "d"
	case AreaResidential:
		return "e"
	case AreaPureResidential:
		return "f"
	case AreaHealthcare:
		return "g"
	default:
		return ""
	}
}

// ParseAreaCategory parses a raw string into an AreaCategory.
// Accepts English keys, German names, and normalized variants.
func ParseAreaCategory(raw string) (AreaCategory, error) {
	switch normalizeCategory(raw) {
	case "industrial", "industriegebiet":
		return AreaIndustrial, nil
	case "commercial", "gewerbegebiet":
		return AreaCommercial, nil
	case "urban", "urbanesgebiet":
		return AreaUrban, nil
	case "mixed", "mischgebiet", "kerngebiet", "dorfgebiet":
		return AreaMixed, nil
	case "residential", "wohngebiet", "allgemeineswohngebiet", "kleinsiedlungsgebiet":
		return AreaResidential, nil
	case "pureresidential", "reineswohngebiet":
		return AreaPureResidential, nil
	case "healthcare", "kurgebiet", "krankenhaus", "pflegeanstalt":
		return AreaHealthcare, nil
	default:
		return "", fmt.Errorf("unknown TA Lärm area category %q", raw)
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

// IsErhoehtEmpfindlichkeit returns true for area categories with increased sensitivity
// per TA Lärm Nr. 6.5 (2017 text): d) Mixed, e) Residential, f) PureResidential.
func IsErhoehtEmpfindlichkeit(category AreaCategory) bool {
	switch category {
	case AreaMixed, AreaResidential, AreaPureResidential:
		return true
	default:
		return false
	}
}
