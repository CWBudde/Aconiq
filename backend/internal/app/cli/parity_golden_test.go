package cli

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/qa/golden"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
)

// The cross-target parity contract.
//
// testdata/parity/*.geojson and their goldens are read from *two* trees: this
// test writes them, and frontend/src/api/browser-parity.test.ts reads them to
// check that browser mode builds the same scene out of the same model. Moving,
// renaming or reshaping anything under testdata/parity/ breaks that test, which
// no Go tool will tell you about — grep the frontend before you do.
//
// What this file pins is deliberately narrow: the levels the *CLI* computes from
// a GeoJSON model, by running the CLI's own unexported extraction rather than a
// paraphrase of it. That is why it lives in package cli at all. Normative
// evidence is the acceptance suite's job (internal/qa/acceptance); these
// goldens answer "do both targets read the same model the same way".
//
// The two fixtures cover what a name-level parity suite cannot see, and both
// defects they encode were real:
//
//   - road_building_barrier carries a MultiPolygon building, which the browser
//     dropped whole until the pr-fix round on PR #19;
//   - parking_building carries a Parkplatz polygon *with a hole*, which reaches
//     the hole-aware geo.PolygonCentroid from the same round, plus a facade so
//     the Nr. 3.6 mirrored paths are exercised.
//
// Each fixture also carries `id` twice — at feature level and in properties —
// because the Go normalizer prefers the property while the frontend normalizer
// reads only the feature-level one. Without both, the two targets would name
// the same feature differently and no comparison would be possible.
//
// Regenerate with `just update-golden`.

// parityFixtures is the fixture set, named once so the Go tests and the
// frontend test cannot drift apart silently.
var parityFixtures = []string{"road_building_barrier", "parking_building"}

// parityRunParams is the parameter map both targets must be driven with. Every
// value is stated rather than defaulted: a default that differs between the CLI
// and the browser is exactly the kind of divergence this contract exists to
// catch, and it cannot be caught by a test that relies on one.
var parityRunParams = map[string]string{
	"surface_type":       "SMA",
	"speed_pkw_kph":      "100",
	"speed_lkw1_kph":     "80",
	"speed_lkw2_kph":     "70",
	"speed_krad_kph":     "100",
	"gradient_percent":   "0",
	"traffic_day_pkw":    "900",
	"traffic_day_lkw1":   "40",
	"traffic_day_lkw2":   "60",
	"traffic_day_krad":   "10",
	"traffic_night_pkw":  "200",
	"traffic_night_lkw1": "10",
	"traffic_night_lkw2": "20",
	"traffic_night_krad": "2",
	"segment_length_m":   "5",
	"min_distance_m":     "3",
	"receiver_height_m":  "4",
	"grid_resolution_m":  "10",
	"grid_padding_m":     "50",
}

// parityReceiverSnapshot mirrors rls19_test20.ReceiverSnapshot rather than
// inventing a second shape, so the frontend has one snapshot format to read.
type parityReceiverSnapshot struct {
	ID      string  `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	HeightM float64 `json:"height_m"`
	LrDay   float64 `json:"lr_day"`
	LrNight float64 `json:"lr_night"`
}

type paritySnapshotFile struct {
	Receivers []parityReceiverSnapshot `json:"receivers"`
}

// parityScene is one fixture read into the pieces ComputeReceiverOutputs takes,
// so a test can drop one of them and see whether it was doing anything.
type parityScene struct {
	roadSources []rls19road.RoadSource
	barriers    []rls19road.Barrier
	receivers   []geo.PointReceiver
	config      rls19road.PropagationConfig
}

func TestParityFixturesMatchTheirGoldens(t *testing.T) {
	t.Parallel()

	for _, name := range parityFixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			snapshot := snapshotFromOutputs(loadParityScene(t, name).compute(t))

			if len(snapshot.Receivers) == 0 {
				t.Fatal("fixture produced no receiver outputs")
			}

			golden.AssertJSONSnapshot(t,
				filepath.Join("testdata", "parity", name+".golden.json"), snapshot)
		})
	}
}

// loadParityScene runs the same sequence runRLS19RoadModule does, with receivers
// taken from the model (the `custom` receiver mode) so the browser can be
// pointed at the same points in the same order.
func loadParityScene(t *testing.T, name string) parityScene {
	t.Helper()

	modelPath := filepath.Join("testdata", "parity", name+".geojson")

	model, err := loadValidatedModel(modelPath, "EPSG:25832", modelPath)
	if err != nil {
		t.Fatalf("load %s: %v", modelPath, err)
	}

	options, err := parseRLS19RoadRunOptions(parityRunParams)
	if err != nil {
		t.Fatalf("parse run options: %v", err)
	}

	roadSources, _, err := extractRLS19RoadSources(model, options, []string{"line", "area"})
	if err != nil {
		t.Fatalf("extract road sources: %v", err)
	}

	barriers, err := extractRLS19Barriers(model)
	if err != nil {
		t.Fatalf("extract barriers: %v", err)
	}

	buildings, err := extractRLS19Buildings(model)
	if err != nil {
		t.Fatalf("extract buildings: %v", err)
	}

	parkingSources, _, err := extractRLS19ParkingSources(model)
	if err != nil {
		t.Fatalf("extract parking sources: %v", err)
	}

	receivers, err := extractExplicitReceivers(model)
	if err != nil {
		t.Fatalf("extract receivers: %v", err)
	}

	config := options.PropagationConfig()
	config.Buildings = buildings
	config.ParkingSources = parkingSources

	return parityScene{
		roadSources: roadSources,
		barriers:    barriers,
		receivers:   receivers,
		config:      config,
	}
}

func (scene parityScene) compute(t *testing.T) []rls19road.ReceiverOutput {
	t.Helper()

	outputs, err := rls19road.ComputeReceiverOutputs(
		scene.receivers, scene.roadSources, scene.barriers, scene.config,
	)
	if err != nil {
		t.Fatalf("compute receiver outputs: %v", err)
	}

	return outputs
}

func snapshotFromOutputs(outputs []rls19road.ReceiverOutput) paritySnapshotFile {
	snapshot := paritySnapshotFile{Receivers: make([]parityReceiverSnapshot, 0, len(outputs))}
	for _, output := range outputs {
		snapshot.Receivers = append(snapshot.Receivers, parityReceiverSnapshot{
			ID:      output.Receiver.ID,
			X:       parityRound6(output.Receiver.Point.X),
			Y:       parityRound6(output.Receiver.Point.Y),
			HeightM: parityRound6(output.Receiver.HeightM),
			LrDay:   parityRound6(output.Indicators.LrDay),
			LrNight: parityRound6(output.Indicators.LrNight),
		})
	}

	return snapshot
}

func parityRound6(value float64) float64 {
	return math.Round(value*1e6) / 1e6
}

// TestParityFixturesReachTheScenesTheyClaim keeps the fixtures from quietly
// degrading into plain road models. A MultiPolygon building that stopped
// producing two barriers, or a Parkplatz that stopped being extracted, would
// still produce a golden — just a golden of the wrong scene, against which the
// browser could agree perfectly while both were wrong.
func TestParityFixturesReachTheScenesTheyClaim(t *testing.T) {
	t.Parallel()

	road := loadParityScene(t, "road_building_barrier")

	if len(road.roadSources) != 2 {
		t.Errorf("extracted %d road sources, want 2", len(road.roadSources))
	}

	// One MultiPolygon feature, two parts, two buildings.
	if len(road.config.Buildings) != 2 {
		t.Errorf("MultiPolygon building expanded to %d buildings, want 2",
			len(road.config.Buildings))
	}

	parking := loadParityScene(t, "parking_building")

	if len(parking.config.ParkingSources) != 2 {
		t.Fatalf("extracted %d Parkplatz sources, want 2",
			len(parking.config.ParkingSources))
	}

	// The Rastanlage ring has a courtyard, so its area is the exterior less the
	// hole: 100 x 50 − 30 x 20 = 4400 m². A centroid-and-area pair that ignored
	// the hole would report 5000.
	//
	// The courtyard is deliberately off-centre. AreaM2 is validated but never
	// enters the level — only the centroid does — so a hole centred inside the
	// lot would subtract area while moving nothing a receiver could hear, and
	// the browser-side parity test would not be able to see it at all.
	const wantAreaM2 = 4400.0

	if math.Abs(parking.config.ParkingSources[0].AreaM2-wantAreaM2) > 1e-6 {
		t.Errorf("Parkplatz area = %.6f m², want %.6f — the hole is not being subtracted",
			parking.config.ParkingSources[0].AreaM2, wantAreaM2)
	}
}

// TestParityFixtureScenesAreLoadBearing removes one scene element at a time and
// requires the level to move. A fixture whose barrier shields nothing and whose
// buildings reflect nothing still produces a stable golden the browser can
// match — and proves nothing about either target reading them.
func TestParityFixtureScenesAreLoadBearing(t *testing.T) {
	t.Parallel()

	road := loadParityScene(t, "road_building_barrier")
	roadBase := levelsByID(road.compute(t))

	withoutBarriers := road
	withoutBarriers.barriers = nil
	assertEveryLevelMoves(t, "barriers", roadBase, levelsByID(withoutBarriers.compute(t)))

	withoutBuildings := road
	withoutBuildings.config.Buildings = nil
	assertEveryLevelMoves(t, "buildings", roadBase, levelsByID(withoutBuildings.compute(t)))

	parking := loadParityScene(t, "parking_building")
	parkingBase := levelsByID(parking.compute(t))

	// Removing the Parkplätze leaves that fixture with no source of any kind,
	// and PR #19's refusal of the quiet zeros is what answers: a scene with
	// nothing in it must not compute a level. That refusal is the strongest
	// available statement that the lots are the only thing feeding these
	// receivers.
	withoutParking := parking
	withoutParking.config.ParkingSources = nil

	_, err := rls19road.ComputeReceiverOutputs(
		withoutParking.receivers, withoutParking.roadSources,
		withoutParking.barriers, withoutParking.config,
	)
	if err == nil {
		t.Error("a scene with neither road nor Parkplatz sources computed a level")
	}

	withoutFacade := parking
	withoutFacade.config.Buildings = nil
	assertEveryLevelMoves(t, "facade", parkingBase, levelsByID(withoutFacade.compute(t)))
}

func levelsByID(outputs []rls19road.ReceiverOutput) map[string]float64 {
	levels := make(map[string]float64, len(outputs))
	for _, output := range outputs {
		levels[output.Receiver.ID] = output.Indicators.LrDay
	}

	return levels
}

func assertEveryLevelMoves(t *testing.T, element string, base, without map[string]float64) {
	t.Helper()

	for id, level := range base {
		other, ok := without[id]
		if !ok {
			t.Fatalf("receiver %q vanished when %s were removed", id, element)
		}

		if math.Abs(level-other) < 1e-3 {
			t.Errorf("removing the %s left receiver %q at %.6f dB — the fixture does not reach them",
				element, id, level)
		}
	}
}
