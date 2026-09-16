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
// The three fixtures cover what a name-level parity suite cannot see, and every
// defect they encode was real:
//
//   - road_building_barrier carries a MultiPolygon building, which the browser
//     dropped whole until the pr-fix round on PR #19;
//   - parking_building carries a Parkplatz polygon *with a hole*, which reaches
//     the hole-aware geo.PolygonCentroid from the same round, plus a facade so
//     the Nr. 3.6 mirrored paths are exercised;
//   - road_geographic is road_building_barrier's scene re-expressed in
//     EPSG:4326 over a site near Hannover, and it is the only fixture that can
//     see a target computing in degrees. Its coordinates are the metric
//     fixture's own offsets around ETRS89 / UTM 32N (550000, 5800000),
//     inverse-projected into WGS84 lon/lat. Projecting them forward reproduces
//     that shape to about a millimetre; the scene as a whole lands half a metre
//     from where it started, because the ETRS89↔WGS84 leg carries an
//     ellipsoidal height this pipeline does not, and that is a near-rigid shift
//     of everything at once rather than a change to any distance in the model.
//     A target that skipped the projection would instead read the whole scene
//     as 0.002 units across, put every propagation distance under the modules'
//     minimum-distance clamp, and report each receiver at the source's emission
//     level — which is what TestGeographicFixtureSeesTheDegreeDefect measures.
//
// Each fixture also carries `id` twice — at feature level and in properties —
// because the Go normalizer prefers the property while the frontend normalizer
// reads only the feature-level one. Without both, the two targets would name
// the same feature differently and no comparison would be possible.
//
// Regenerate with `just update-golden`.

// parityFixture is one fixture together with the project CRS it is authored in.
// The CRS is per fixture rather than a constant because the compute projection
// is part of what the two targets have to agree on: a fixture in degrees and a
// fixture in metres must both reach the kernel in metres, and only the fixture
// knows which of the two it is.
type parityFixture struct {
	name       string
	projectCRS string
}

// parityFixtures is the fixture set, named once so the Go tests and the
// frontend test cannot drift apart silently.
var parityFixtures = []parityFixture{
	{name: "road_building_barrier", projectCRS: "EPSG:25832"},
	{name: "parking_building", projectCRS: "EPSG:25832"},
	{name: "road_geographic", projectCRS: "EPSG:4326"},
}

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
	projection  computeProjection
}

func TestParityFixturesMatchTheirGoldens(t *testing.T) {
	t.Parallel()

	for _, fixture := range parityFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()

			snapshot := snapshotFromOutputs(loadParityScene(t, fixture).compute(t))

			if len(snapshot.Receivers) == 0 {
				t.Fatal("fixture produced no receiver outputs")
			}

			golden.AssertJSONSnapshot(t,
				filepath.Join("testdata", "parity", fixture.name+".golden.json"), snapshot)
		})
	}
}

// loadParityScene runs the same sequence runRLS19RoadModule does, with receivers
// taken from the model (the `custom` receiver mode) so the browser can be
// pointed at the same points in the same order.
//
// resolveComputeModel is part of that sequence and was missing here: this
// helper used to load every fixture as EPSG:25832 and hand the model straight
// to extraction. That was harmless while both fixtures were metric and would
// have been silently wrong the moment one was not — the geographic fixture
// would have pinned a golden of the defect rather than of the fix.
func loadParityScene(t *testing.T, fixture parityFixture) parityScene {
	t.Helper()

	modelPath := filepath.Join("testdata", "parity", fixture.name+".geojson")

	loaded, err := loadValidatedModel(modelPath, fixture.projectCRS, modelPath)
	if err != nil {
		t.Fatalf("load %s: %v", modelPath, err)
	}

	model, projection, err := resolveComputeModel(loaded, fixture.projectCRS)
	if err != nil {
		t.Fatalf("resolve compute model for %s: %v", modelPath, err)
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
		projection:  projection,
	}
}

// parityFixtureNamed looks a fixture up by name, so the tests below can reach
// one without restating its project CRS — which would be a second place to get
// it wrong.
func parityFixtureNamed(t *testing.T, name string) parityFixture {
	t.Helper()

	for _, fixture := range parityFixtures {
		if fixture.name == name {
			return fixture
		}
	}

	t.Fatalf("no parity fixture named %q", name)

	return parityFixture{}
}

func loadParitySceneNamed(t *testing.T, name string) parityScene {
	t.Helper()

	return loadParityScene(t, parityFixtureNamed(t, name))
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

	road := loadParitySceneNamed(t, "road_building_barrier")

	if len(road.roadSources) != 2 {
		t.Errorf("extracted %d road sources, want 2", len(road.roadSources))
	}

	// One MultiPolygon feature, two parts, two buildings.
	if len(road.config.Buildings) != 2 {
		t.Errorf("MultiPolygon building expanded to %d buildings, want 2",
			len(road.config.Buildings))
	}

	parking := loadParitySceneNamed(t, "parking_building")

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

	road := loadParitySceneNamed(t, "road_building_barrier")
	roadBase := levelsByID(road.compute(t))

	withoutBarriers := road
	withoutBarriers.barriers = nil
	assertEveryLevelMoves(t, "barriers", roadBase, levelsByID(withoutBarriers.compute(t)))

	withoutBuildings := road
	withoutBuildings.config.Buildings = nil
	assertEveryLevelMoves(t, "buildings", roadBase, levelsByID(withoutBuildings.compute(t)))

	parking := loadParitySceneNamed(t, "parking_building")
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

// TestGeographicFixtureIsActuallyProjected keeps road_geographic in degrees.
//
// The fixture only means anything while it is authored in EPSG:4326: re-write
// its coordinates in metres and it still produces a golden, still matches, and
// silently stops being the one fixture that can see a target computing in
// degrees. So the projection itself is asserted, not just its result.
func TestGeographicFixtureIsActuallyProjected(t *testing.T) {
	t.Parallel()

	scene := loadParitySceneNamed(t, "road_geographic")

	if !scene.projection.Applied {
		t.Fatal("road_geographic was not projected — it is no longer authored in a geographic CRS")
	}

	if scene.projection.ProjectCRS != "EPSG:4326" {
		t.Errorf("project CRS = %q, want EPSG:4326", scene.projection.ProjectCRS)
	}

	if scene.projection.ComputeCRS != "EPSG:25832" {
		t.Errorf("compute CRS = %q, want EPSG:25832 — the site sits in UTM zone 32",
			scene.projection.ComputeCRS)
	}

	// Eastings and northings, not longitudes and latitudes. A false-positive
	// Applied over coordinates nothing had moved would still fail here.
	for _, receiver := range scene.receivers {
		if receiver.Point.X < 400_000 || receiver.Point.X > 700_000 {
			t.Errorf("receiver %q easting %.3f is not an ETRS89 / UTM 32N easting",
				receiver.ID, receiver.Point.X)
		}

		if receiver.Point.Y < 5_000_000 || receiver.Point.Y > 6_500_000 {
			t.Errorf("receiver %q northing %.3f is not an ETRS89 / UTM 32N northing",
				receiver.ID, receiver.Point.Y)
		}
	}
}

// TestGeographicFixtureSeesTheDegreeDefect is the fixture's reason to exist: a
// fixture that passes with and without the projection proves nothing.
//
// It rebuilds the same scene from the *unprojected* model — which is exactly
// what browser mode did before this change — and requires the levels to be tens
// of dB apart. In degrees the whole scene is 0.002 units across, so every
// propagation distance falls under min_distance_m and each receiver reports the
// source's emission level verbatim.
func TestGeographicFixtureSeesTheDegreeDefect(t *testing.T) {
	t.Parallel()

	const minimumDefectDb = 10.0

	projected := levelsByID(loadParitySceneNamed(t, "road_geographic").compute(t))
	inDegrees := levelsByID(loadUnprojectedParityScene(t, "road_geographic").compute(t))

	for id, want := range projected {
		got, ok := inDegrees[id]
		if !ok {
			t.Fatalf("receiver %q is missing from the unprojected scene", id)
		}

		// Logged rather than only asserted: `go test -v` then states the size of
		// the defect this fixture sees, which is the evidence a reader wants
		// when they ask what the projection is worth.
		t.Logf("receiver %q: projected %.3f dB, in degrees %.3f dB, delta %.3f dB",
			id, want, got, math.Abs(want-got))

		if math.Abs(want-got) < minimumDefectDb {
			t.Errorf("receiver %q reads %.3f dB projected and %.3f dB in degrees — "+
				"a difference of %.3f dB, under the %.0f dB this fixture has to be able to see",
				id, want, got, math.Abs(want-got), minimumDefectDb)
		}
	}
}

// loadUnprojectedParityScene builds a scene the way the pipeline did before
// resolveComputeModel existed: straight off the loaded model, in whatever units
// the file is authored in. It exists for the test above and for nothing else.
func loadUnprojectedParityScene(t *testing.T, name string) parityScene {
	t.Helper()

	fixture := parityFixtureNamed(t, name)
	modelPath := filepath.Join("testdata", "parity", fixture.name+".geojson")

	model, err := loadValidatedModel(modelPath, fixture.projectCRS, modelPath)
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

	receivers, err := extractExplicitReceivers(model)
	if err != nil {
		t.Fatalf("extract receivers: %v", err)
	}

	config := options.PropagationConfig()
	config.Buildings = buildings

	return parityScene{
		roadSources: roadSources,
		barriers:    barriers,
		receivers:   receivers,
		config:      config,
	}
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
