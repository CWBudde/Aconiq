package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// decodeNormativeModel builds a model the way loadValidatedModel would, so the
// property values are the `any` shapes JSON actually produces rather than
// hand-written Go literals that would hide a decoding bug.
func decodeNormativeModel(t *testing.T, features string) modelgeojson.Model {
	t.Helper()

	var model modelgeojson.Model

	err := json.Unmarshal([]byte(`{"features":`+features+`}`), &model)
	if err != nil {
		t.Fatalf("decode model: %v", err)
	}

	return model
}

const normativeTrackFeature = `{
	"id": "track-1",
	"kind": "source",
	"source_type": "line",
	"geometry_type": "LineString",
	"coordinates": [[0, 0], [100, 0]],
	"properties": {
		"schall03_strecke_max_kph": 160,
		"schall03_fahrbahn": "feste-fahrbahn",
		"schall03_surface": "bug",
		"schall03_bridge_type": 2,
		"schall03_is_station": true,
		"schall03_operations": [
			{"zugart": "ICE-3-Vollzug", "trains_per_hour_day": 2, "trains_per_hour_night": 1}
		]
	}
}`

func TestExtractSchall03NormativeSceneReadsTrackProperties(t *testing.T) {
	t.Parallel()

	model := decodeNormativeModel(t, "["+normativeTrackFeature+"]")

	scene, err := extractSchall03NormativeScene(model, []string{"line"})
	if err != nil {
		t.Fatalf("extract normative scene: %v", err)
	}

	if len(scene.Segments) != 1 {
		t.Fatalf("got %d segments, want 1", len(scene.Segments))
	}

	segment := scene.Segments[0]
	if segment.ID != "track-1" {
		t.Fatalf("segment ID = %q, want track-1", segment.ID)
	}

	if segment.Fahrbahn != schall03.FahrbahnartFesteFahrbahn {
		t.Fatalf("Fahrbahn = %v, want FesteFahrbahn", segment.Fahrbahn)
	}

	if segment.Surface != schall03.SurfaceCondBuG {
		t.Fatalf("Surface = %v, want BuG", segment.Surface)
	}

	if segment.BridgeType != 2 || !segment.IsStation || segment.StreckeMaxKPH != 160 {
		t.Fatalf("unexpected segment infrastructure: %+v", segment)
	}

	if len(segment.Operations) != 1 || segment.Operations[0].TrainType != "ICE-3-Vollzug" {
		t.Fatalf("unexpected operations: %+v", segment.Operations)
	}

	// The Zugart supplies the composition and the default speed.
	if segment.Operations[0].SpeedKPH != 300 {
		t.Fatalf("SpeedKPH = %v, want the ICE-3-Vollzug default 300", segment.Operations[0].SpeedKPH)
	}
}

// An omitted Fahrbahnart must land on the reference row, not on whatever the
// enum happens to number zero at the time.
func TestExtractSchall03NormativeSceneDefaultsToTheReferenceTrackType(t *testing.T) {
	t.Parallel()

	model := decodeNormativeModel(t, `[{
		"id": "track-1",
		"kind": "source",
		"source_type": "line",
		"geometry_type": "LineString",
		"coordinates": [[0, 0], [100, 0]],
		"properties": {
			"schall03_strecke_max_kph": 120,
			"schall03_operations": [
				{"fz_composition": [{"fz": 7, "count": 1}, {"fz": 10, "count": 24}],
				 "speed_kph": 100, "trains_per_hour_day": 1, "trains_per_hour_night": 2}
			]
		}
	}]`)

	scene, err := extractSchall03NormativeScene(model, []string{"line"})
	if err != nil {
		t.Fatalf("extract normative scene: %v", err)
	}

	segment := scene.Segments[0]
	if segment.Fahrbahn != schall03.FahrbahnartSchwellengleis || segment.SFahrbahn != schall03.SFahrbahnSchwellengleis {
		t.Fatalf("omitted track type did not default to Schwellengleis: %+v", segment)
	}

	if segment.Surface != schall03.SurfaceCondNone || segment.BridgeType != 0 || segment.CurveRadiusM != 0 {
		t.Fatalf("omitted properties did not default to the uncorrected case: %+v", segment)
	}

	composition := segment.Operations[0].FzComposition
	if len(composition) != 2 || composition[0].Fz != 7 || composition[1].Count != 24 {
		t.Fatalf("unexpected Fz composition: %+v", composition)
	}
}

func TestExtractSchall03NormativeSceneRejectsIncompleteTracks(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		properties string
		wantSubstr string
	}{
		"missing Streckenhöchstgeschwindigkeit": {
			`"schall03_operations": [{"zugart": "S-Bahn", "trains_per_hour_day": 1, "trains_per_hour_night": 1}]`,
			"schall03_strecke_max_kph",
		},
		"unknown Zugart": {
			`"schall03_strecke_max_kph": 120, "schall03_operations": [{"zugart": "Orient-Express", "trains_per_hour_day": 1, "trains_per_hour_night": 1}]`,
			"unknown Zugart",
		},
		"unknown Fahrbahnart": {
			`"schall03_strecke_max_kph": 120, "schall03_fahrbahn": "schotterbett", "schall03_operations": [{"zugart": "S-Bahn", "trains_per_hour_day": 1, "trains_per_hour_night": 1}]`,
			"unknown Fahrbahnart",
		},
		"neither Zugart nor composition": {
			`"schall03_strecke_max_kph": 120, "schall03_operations": [{"trains_per_hour_day": 1, "trains_per_hour_night": 1}]`,
			`requires either "zugart" or "fz_composition"`,
		},
		"empty operations array": {
			`"schall03_strecke_max_kph": 120, "schall03_operations": []`,
			"non-empty array",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			model := decodeNormativeModel(t, `[{
				"id": "track-1", "kind": "source", "source_type": "line",
				"geometry_type": "LineString", "coordinates": [[0, 0], [100, 0]],
				"properties": {`+testCase.properties+`}
			}]`)

			_, err := extractSchall03NormativeScene(model, []string{"line"})
			if err == nil {
				t.Fatal("expected extraction to fail")
			}

			if !strings.Contains(err.Error(), testCase.wantSubstr) {
				t.Fatalf("error %q does not mention %q", err, testCase.wantSubstr)
			}
		})
	}
}

// A barrier shields by default. It also reflects only when it says it does,
// because Gl. 20 treats the two effects separately.
func TestExtractSchall03NormativeSceneSplitsShieldingFromReflection(t *testing.T) {
	t.Parallel()

	model := decodeNormativeModel(t, `[`+normativeTrackFeature+`, {
		"id": "wall-1", "kind": "barrier", "height_m": 4,
		"geometry_type": "LineString", "coordinates": [[0, 10], [50, 10], [100, 10]],
		"properties": {"schall03_reflective": true, "schall03_base_height_m": 1, "schall03_wall_surface": "absorbing"}
	}, {
		"id": "wall-2", "kind": "barrier", "height_m": 3,
		"geometry_type": "LineString", "coordinates": [[0, -10], [100, -10]],
		"properties": {}
	}]`)

	scene, err := extractSchall03NormativeScene(model, []string{"line"})
	if err != nil {
		t.Fatalf("extract normative scene: %v", err)
	}

	// Two panels from wall-1's three vertices, one from wall-2.
	if len(scene.Barriers) != 3 {
		t.Fatalf("got %d barrier segments, want 3: %+v", len(scene.Barriers), scene.Barriers)
	}

	// Only the reflective barrier's two panels become walls.
	if len(scene.Walls) != 2 {
		t.Fatalf("got %d reflecting walls, want 2: %+v", len(scene.Walls), scene.Walls)
	}

	for _, wall := range scene.Walls {
		if wall.Surface != schall03.WallSurfaceAbsorbing {
			t.Fatalf("wall surface = %v, want absorbing", wall.Surface)
		}
	}

	if !scene.Barriers[0].Reflective || scene.Barriers[0].BaseHeightM != 1 {
		t.Fatalf("reflective barrier lost its Gl. 20 inputs: %+v", scene.Barriers[0])
	}

	if scene.Barriers[2].Reflective {
		t.Fatalf("an unannotated barrier must stay absorbing: %+v", scene.Barriers[2])
	}
}

// Buildings reflect only on request; adding reflection while they still do not
// shield would raise every level behind a building.
func TestExtractSchall03NormativeSceneTreatsBuildingReflectionAsOptIn(t *testing.T) {
	t.Parallel()

	const footprint = `"geometry_type": "Polygon", "coordinates": [[[0, 20], [10, 20], [10, 30], [0, 30], [0, 20]]]`

	silent := decodeNormativeModel(t, `[`+normativeTrackFeature+`, {
		"id": "house-1", "kind": "building", "height_m": 8, `+footprint+`, "properties": {}
	}]`)

	scene, err := extractSchall03NormativeScene(silent, []string{"line"})
	if err != nil {
		t.Fatalf("extract normative scene: %v", err)
	}

	if len(scene.Walls) != 0 || len(scene.Barriers) != 0 {
		t.Fatalf("an unannotated building must contribute nothing: %+v", scene)
	}

	reflecting := decodeNormativeModel(t, `[`+normativeTrackFeature+`, {
		"id": "house-1", "kind": "building", "height_m": 8, `+footprint+`,
		"properties": {"schall03_reflecting_wall": true}
	}]`)

	scene, err = extractSchall03NormativeScene(reflecting, []string{"line"})
	if err != nil {
		t.Fatalf("extract normative scene: %v", err)
	}

	if len(scene.Walls) != 4 {
		t.Fatalf("got %d walls from a four-sided footprint, want 4: %+v", len(scene.Walls), scene.Walls)
	}

	// Tabelle 18 row "Gebäudewände mit Fenstern und kleinen Anbauten".
	for _, wall := range scene.Walls {
		if wall.Surface != schall03.WallSurfaceBuilding {
			t.Fatalf("wall surface = %v, want the building default", wall.Surface)
		}
	}

	if len(scene.Barriers) != 0 {
		t.Fatalf("buildings must not become shielding obstacles in this slice: %+v", scene.Barriers)
	}
}

func TestResolveSchall03Engine(t *testing.T) {
	t.Parallel()

	normative := decodeNormativeModel(t, "["+normativeTrackFeature+"]")
	preview := decodeNormativeModel(t, `[{
		"id": "track-1", "kind": "source", "source_type": "line",
		"geometry_type": "LineString", "coordinates": [[0, 0], [100, 0]],
		"properties": {"rail_train_class": "passenger"}
	}]`)

	for name, testCase := range map[string]struct {
		configured string
		model      modelgeojson.Model
		want       string
		wantErr    bool
	}{
		"auto with normative data":       {schall03.EngineAuto, normative, schall03.EngineNormative, false},
		"auto without normative data":    {schall03.EngineAuto, preview, "", true},
		"normative without data":         {schall03.EngineNormative, preview, "", true},
		"preview always available":       {schall03.EnginePreview, preview, schall03.EnginePreview, false},
		"preview over a normative model": {schall03.EnginePreview, normative, schall03.EnginePreview, false},
		"unknown value":                  {"anlage2", normative, "", true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveSchall03Engine(testCase.configured, testCase.model)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got engine %q", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("resolve engine: %v", err)
			}

			if got != testCase.want {
				t.Fatalf("engine = %q, want %q", got, testCase.want)
			}
		})
	}
}
