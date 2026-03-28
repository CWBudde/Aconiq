package talaerm

import (
	"testing"
)

func TestThresholdsOutdoor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		category AreaCategory
		day      int
		night    int
	}{
		{AreaIndustrial, 70, 70},
		{AreaCommercial, 65, 50},
		{AreaUrban, 63, 45},
		{AreaMixed, 60, 45},
		{AreaResidential, 55, 40},
		{AreaPureResidential, 50, 35},
		{AreaHealthcare, 45, 35},
	}

	for _, tc := range cases {
		got, err := ThresholdsOutdoor(tc.category)
		if err != nil {
			t.Fatalf("thresholds for %s: %v", tc.category, err)
		}

		if got.Day != tc.day || got.Night != tc.night {
			t.Fatalf("%s: got %d/%d want %d/%d", tc.category, got.Day, got.Night, tc.day, tc.night)
		}
	}
}

func TestThresholdsOutdoorUnknown(t *testing.T) {
	t.Parallel()

	_, err := ThresholdsOutdoor("bogus")
	if err == nil {
		t.Fatal("expected error for unknown category")
	}
}

func TestThresholdsIndoor(t *testing.T) {
	t.Parallel()

	got := ThresholdsIndoor()
	if got.Day != 35 || got.Night != 25 {
		t.Fatalf("indoor: got %d/%d want 35/25", got.Day, got.Night)
	}
}

func TestThresholdsRareEvents(t *testing.T) {
	t.Parallel()

	got := ThresholdsRareEvents()
	if got.Day != 70 || got.Night != 55 {
		t.Fatalf("rare events: got %d/%d want 70/55", got.Day, got.Night)
	}
}

func TestPeakLimitsOutdoor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		category AreaCategory
		day      int
		night    int
	}{
		{AreaIndustrial, 100, 90},
		{AreaCommercial, 95, 70},
		{AreaUrban, 93, 65},
		{AreaMixed, 90, 65},
		{AreaResidential, 85, 60},
		{AreaPureResidential, 80, 55},
		{AreaHealthcare, 75, 55},
	}

	for _, tc := range cases {
		got, err := PeakLimitsOutdoor(tc.category)
		if err != nil {
			t.Fatalf("peak limits for %s: %v", tc.category, err)
		}

		if got.Day != tc.day || got.Night != tc.night {
			t.Fatalf("%s: got %d/%d want %d/%d", tc.category, got.Day, got.Night, tc.day, tc.night)
		}
	}
}

func TestPeakLimitsIndoor(t *testing.T) {
	t.Parallel()

	got := PeakLimitsIndoor()
	if got.Day != 45 || got.Night != 35 {
		t.Fatalf("indoor peak: got %d/%d want 45/35", got.Day, got.Night)
	}
}

func TestPeakLimitsRareEvents(t *testing.T) {
	t.Parallel()

	t.Run("commercial", func(t *testing.T) {
		t.Parallel()

		got, err := PeakLimitsRareEvents(AreaCommercial)
		if err != nil {
			t.Fatalf("rare events commercial: %v", err)
		}

		if got.Day != 95 || got.Night != 70 {
			t.Fatalf("commercial: got %d/%d want 95/70", got.Day, got.Night)
		}
	})

	t.Run("categories_c_through_g", func(t *testing.T) {
		t.Parallel()

		for _, cat := range []AreaCategory{AreaUrban, AreaMixed, AreaResidential, AreaPureResidential, AreaHealthcare} {
			got, err := PeakLimitsRareEvents(cat)
			if err != nil {
				t.Fatalf("rare events %s: %v", cat, err)
			}

			if got.Day != 90 || got.Night != 65 {
				t.Fatalf("%s: got %d/%d want 90/65", cat, got.Day, got.Night)
			}
		}
	})

	t.Run("industrial_not_applicable", func(t *testing.T) {
		t.Parallel()

		_, err := PeakLimitsRareEvents(AreaIndustrial)
		if err == nil {
			t.Fatal("expected error for industrial area")
		}
	})
}

func TestParseAreaCategory(t *testing.T) {
	t.Parallel()

	cases := map[string]AreaCategory{
		// English keys
		"industrial":       AreaIndustrial,
		"commercial":       AreaCommercial,
		"urban":            AreaUrban,
		"mixed":            AreaMixed,
		"residential":      AreaResidential,
		"pure_residential": AreaPureResidential,
		"healthcare":       AreaHealthcare,
		// German names
		"Industriegebiet":      AreaIndustrial,
		"Gewerbegebiet":        AreaCommercial,
		"urbanes Gebiet":       AreaUrban,
		"Kerngebiet":           AreaMixed,
		"Dorfgebiet":           AreaMixed,
		"Mischgebiet":          AreaMixed,
		"Wohngebiet":           AreaResidential,
		"Kleinsiedlungsgebiet": AreaResidential,
		"reines Wohngebiet":    AreaPureResidential,
		"Kurgebiet":            AreaHealthcare,
		"Krankenhaus":          AreaHealthcare,
		"Pflegeanstalt":        AreaHealthcare,
	}

	for raw, want := range cases {
		got, err := ParseAreaCategory(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}

		if got != want {
			t.Fatalf("parse %q: got %q want %q", raw, got, want)
		}
	}
}

func TestParseAreaCategoryUnknown(t *testing.T) {
	t.Parallel()

	_, err := ParseAreaCategory("Fantasiegebiet")
	if err == nil {
		t.Fatal("expected error for unknown category")
	}
}

func TestAreaCategoryCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		category AreaCategory
		code     string
	}{
		{AreaIndustrial, "a"},
		{AreaCommercial, "b"},
		{AreaUrban, "c"},
		{AreaMixed, "d"},
		{AreaResidential, "e"},
		{AreaPureResidential, "f"},
		{AreaHealthcare, "g"},
	}

	for _, tc := range cases {
		got := AreaCategoryCode(tc.category)
		if got != tc.code {
			t.Fatalf("%s: got %q want %q", tc.category, got, tc.code)
		}
	}
}

func TestAreaCategoryLabelDE(t *testing.T) {
	t.Parallel()

	cases := []struct {
		category AreaCategory
		label    string
	}{
		{AreaIndustrial, "Industriegebiet"},
		{AreaCommercial, "Gewerbegebiet"},
		{AreaUrban, "urbanes Gebiet"},
		{AreaMixed, "Kern-, Dorf- oder Mischgebiet"},
		{AreaResidential, "allgemeines Wohngebiet oder Kleinsiedlungsgebiet"},
		{AreaPureResidential, "reines Wohngebiet"},
		{AreaHealthcare, "Kurgebiet, Krankenhaus oder Pflegeanstalt"},
	}

	for _, tc := range cases {
		got := AreaCategoryLabelDE(tc.category)
		if got != tc.label {
			t.Fatalf("%s: got %q want %q", tc.category, got, tc.label)
		}
	}
}

func TestIsErhoehtEmpfindlichkeit(t *testing.T) {
	t.Parallel()

	trueCategories := []AreaCategory{AreaMixed, AreaResidential, AreaPureResidential}
	for _, cat := range trueCategories {
		if !IsErhoehtEmpfindlichkeit(cat) {
			t.Fatalf("%s: expected true", cat)
		}
	}

	falseCategories := []AreaCategory{AreaIndustrial, AreaCommercial, AreaUrban, AreaHealthcare}
	for _, cat := range falseCategories {
		if IsErhoehtEmpfindlichkeit(cat) {
			t.Fatalf("%s: expected false", cat)
		}
	}
}
