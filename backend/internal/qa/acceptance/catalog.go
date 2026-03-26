package acceptance

import (
	"path/filepath"
	"sort"
)

// Fixture describes one deterministic acceptance case.
type Fixture struct {
	Name             string
	StandardID       string
	Description      string
	EvidenceClass    string
	Provenance       string
	ScenarioPath     string
	ExpectedJSONPath string
}

// Catalog returns the currently curated deterministic acceptance fixtures.
func Catalog() []Fixture {
	fixtures := append([]Fixture(nil), acceptanceFixturesCore()...)
	fixtures = append(fixtures, acceptanceFixturesRls19()...)

	sort.Slice(fixtures, func(i, j int) bool {
		if fixtures[i].StandardID == fixtures[j].StandardID {
			return fixtures[i].Name < fixtures[j].Name
		}

		return fixtures[i].StandardID < fixtures[j].StandardID
	})

	return fixtures
}

func acceptanceFixturesCore() []Fixture {
	fixtures := append([]Fixture(nil), acceptanceFixturesCnossosRoad()...)
	fixtures = append(fixtures, acceptanceFixturesCnossosRailIndustryISO()...)
	fixtures = append(fixtures, acceptanceFixturesAircraftAndBeb()...)
	fixtures = append(fixtures, acceptanceFixturesBub()...)

	return fixtures
}

func acceptanceFixturesCnossosRoad() []Fixture {
	return []Fixture{
		{
			Name:             "cnossos-road-synthetic-baseline",
			StandardID:       "cnossos-road",
			Description:      "Repo-authored synthetic road scenario used as a deterministic planning acceptance baseline.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("cnossos-road", "road_preview.scenario.json"),
			ExpectedJSONPath: fixturePath("cnossos-road", "road_preview.golden.json"),
		},
		{
			Name:             "cnossos-road-synthetic-contextual",
			StandardID:       "cnossos-road",
			Description:      "Repo-authored synthetic road scenario stressing expanded road categories, junction context, temperature, studded tyres, and vehicle-class splits.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("cnossos-road", "road_contextual.scenario.json"),
			ExpectedJSONPath: fixturePath("cnossos-road", "road_contextual.golden.json"),
		},
		{
			Name:             "cnossos-rail-synthetic-baseline",
			StandardID:       "cnossos-rail",
			Description:      "Repo-authored synthetic rail scenario used as a deterministic planning acceptance baseline.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("cnossos-rail", "rail_preview.scenario.json"),
			ExpectedJSONPath: fixturePath("cnossos-rail", "rail_preview.golden.json"),
		},
		{
			Name:             "cnossos-rail-synthetic-contextual",
			StandardID:       "cnossos-rail",
			Description:      "Repo-authored synthetic rail scenario stressing traction, track type, roughness, braking, bridge, and curve differences.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("cnossos-rail", "rail_contextual.scenario.json"),
			ExpectedJSONPath: fixturePath("cnossos-rail", "rail_contextual.golden.json"),
		},
	}
}

func acceptanceFixturesCnossosRailIndustryISO() []Fixture {
	return []Fixture{
		{
			Name:             "cnossos-industry-synthetic-baseline",
			StandardID:       "cnossos-industry",
			Description:      "Repo-authored synthetic industry scenario used as a deterministic planning acceptance baseline.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("cnossos-industry", "industry_preview.scenario.json"),
			ExpectedJSONPath: fixturePath("cnossos-industry", "industry_preview.golden.json"),
		},
		{
			Name:             "cnossos-industry-synthetic-contextual",
			StandardID:       "cnossos-industry",
			Description:      "Repo-authored synthetic industry scenario stressing source categories, enclosure states, point/area geometry, and screening/reflection differences.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("cnossos-industry", "industry_contextual.scenario.json"),
			ExpectedJSONPath: fixturePath("cnossos-industry", "industry_contextual.golden.json"),
		},
		{
			Name:             "iso9613-synthetic-point-preview",
			StandardID:       "iso9613",
			Description:      "Repo-authored synthetic ISO 9613 preview scenario stressing point-source height, directivity, and meteorology inputs for the narrow point-source scope.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("iso9613", "point_preview.scenario.json"),
			ExpectedJSONPath: fixturePath("iso9613", "point_preview.golden.json"),
		},
		{
			Name:             "iso9613-synthetic-point-contextual",
			StandardID:       "iso9613",
			Description:      "Repo-authored synthetic ISO 9613 preview scenario stressing contextual propagation differences from ground factor, favorable-weather inputs, and barrier attenuation.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("iso9613", "point_contextual.scenario.json"),
			ExpectedJSONPath: fixturePath("iso9613", "point_contextual.golden.json"),
		},
	}
}

func acceptanceFixturesAircraftAndBeb() []Fixture {
	return []Fixture{
		{
			Name:             "cnossos-aircraft-synthetic-baseline",
			StandardID:       "cnossos-aircraft",
			Description:      "Repo-authored synthetic aircraft scenario used as a deterministic planning acceptance baseline.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("cnossos-aircraft", "aircraft_preview.scenario.json"),
			ExpectedJSONPath: fixturePath("cnossos-aircraft", "aircraft_preview.golden.json"),
		},
		{
			Name:             "cnossos-aircraft-synthetic-contextual",
			StandardID:       "cnossos-aircraft",
			Description:      "Repo-authored synthetic aircraft planning scenario stressing procedure type, thrust mode, lateral offset, and arrival/departure context.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("cnossos-aircraft", "aircraft_contextual.scenario.json"),
			ExpectedJSONPath: fixturePath("cnossos-aircraft", "aircraft_contextual.golden.json"),
		},
		{
			Name:             "beb-exposure-synthetic-baseline",
			StandardID:       "beb-exposure",
			Description:      "Repo-authored synthetic building exposure scenario used as a deterministic acceptance baseline.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("beb-exposure", "building_exposure.scenario.json"),
			ExpectedJSONPath: fixturePath("beb-exposure", "building_exposure.golden.json"),
		},
		{
			Name:             "beb-exposure-synthetic-contextual",
			StandardID:       "beb-exposure",
			Description:      "Repo-authored synthetic building exposure scenario stressing height-derived occupancy and max-facade evaluation.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("beb-exposure", "building_exposure_contextual.scenario.json"),
			ExpectedJSONPath: fixturePath("beb-exposure", "building_exposure_contextual.golden.json"),
		},
	}
}

func acceptanceFixturesBub() []Fixture {
	return []Fixture{
		{
			Name:             "bub-road-synthetic-baseline",
			StandardID:       "bub-road",
			Description:      "Repo-authored synthetic road mapping scenario used as a deterministic acceptance baseline.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("bub-road", "road_mapping.scenario.json"),
			ExpectedJSONPath: fixturePath("bub-road", "road_mapping.golden.json"),
		},
		{
			Name:             "bub-road-synthetic-contextual",
			StandardID:       "bub-road",
			Description:      "Repo-authored synthetic road mapping scenario stressing road function, junction context, tyre share, and canyon/intersection differences.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("bub-road", "road_contextual.scenario.json"),
			ExpectedJSONPath: fixturePath("bub-road", "road_contextual.golden.json"),
		},
		{
			Name:             "buf-aircraft-synthetic-baseline",
			StandardID:       "buf-aircraft",
			Description:      "Repo-authored synthetic aircraft mapping scenario used as a deterministic acceptance baseline.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("buf-aircraft", "aircraft_mapping.scenario.json"),
			ExpectedJSONPath: fixturePath("buf-aircraft", "aircraft_mapping.golden.json"),
		},
		{
			Name:             "buf-aircraft-synthetic-contextual",
			StandardID:       "buf-aircraft",
			Description:      "Repo-authored synthetic aircraft mapping scenario stressing procedure type, thrust mode, lateral offset, and arrival/departure context.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("buf-aircraft", "aircraft_contextual.scenario.json"),
			ExpectedJSONPath: fixturePath("buf-aircraft", "aircraft_contextual.golden.json"),
		},
	}
}

func acceptanceFixturesRls19() []Fixture {
	return []Fixture{
		{
			Name:             "rls19-road-synthetic-baseline",
			StandardID:       "rls19-road",
			Description:      "Repo-authored synthetic RLS-19 road scenario used as a deterministic planning acceptance baseline.",
			EvidenceClass:    "license-safe derived acceptance fixture",
			Provenance:       "repo-authored derived scenario aligned to Phase 17 QA runner coverage",
			ScenarioPath:     fixturePath("rls19-road", "road_planning.scenario.json"),
			ExpectedJSONPath: fixturePath("rls19-road", "road_planning.golden.json"),
		},
		{
			Name:             "schall03-rail-synthetic-baseline",
			StandardID:       "schall03",
			Description:      "Repo-authored synthetic Schall 03 rail scenario used as a deterministic planning acceptance baseline.",
			EvidenceClass:    "license-safe synthetic acceptance fixture",
			Provenance:       "repo-authored synthetic scenario",
			ScenarioPath:     fixturePath("schall03", "rail_planning_preview.scenario.json"),
			ExpectedJSONPath: fixturePath("schall03", "rail_planning_preview.golden.json"),
		},
	}
}

func fixturePath(parts ...string) string {
	all := append([]string{"testdata"}, parts...)
	return filepath.Join(all...)
}
