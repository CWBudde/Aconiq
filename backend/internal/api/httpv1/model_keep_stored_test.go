package httpv1

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// renumberingFacadeModel is an axis-aligned building with facade receivers on
// it, in EPSG:25832, at the first probed position where an EPSG:4326 round
// trip renumbers its facade keys. The position is found rather than written
// down because whether a round trip breaks a tie is a matter of the last bit
// of a projection, which need not agree between architectures. Synthetic
// coordinates; no real address.
func renumberingFacadeModel(t *testing.T) modelgeojson.Model {
	t.Helper()

	for i := range 8 {
		for j := range 8 {
			model := facadeModelAt(t, 547000+float64(i)*37.13, 5795000+float64(j)*41.7)

			geographic, err := modelgeojson.Reproject(model, "EPSG:4326")
			if err != nil {
				t.Fatalf("Reproject into EPSG:4326: %v", err)
			}

			back, err := modelgeojson.Reproject(geographic, "EPSG:25832")
			if err != nil {
				t.Fatalf("Reproject back: %v", err)
			}

			if len(movedFacadeReceivers(modelgeojson.Validate(back))) > 0 {
				return model
			}
		}
	}

	t.Fatal("no probed position renumbers on a round trip; move the positions")

	return modelgeojson.Model{}
}

// facadeModelAt is a 20 × 12 m building at (x0, y0) with confirmed RLS-19
// facade receivers every 5 m.
func facadeModelAt(t *testing.T, x0, y0 float64) modelgeojson.Model {
	t.Helper()

	ring := []geo.Point2D{{X: x0, Y: y0}, {X: x0 + 20, Y: y0}, {X: x0 + 20, Y: y0 + 12}, {X: x0, Y: y0 + 12}, {X: x0, Y: y0}}
	building := geo.FacadeBuilding{ID: "b1", Polygons: [][][]geo.Point2D{{ring}}, HeightM: 9, Levels: 3, HeightSource: geo.SlabHeightMeasured}
	set := geo.FacadeReceiverSet{ID: "fs", BuildingIDs: []string{"b1"}, PlacementRule: geo.PlacementRLS19Facade, SpacingM: 5}

	receivers, err := set.Generate([]geo.FacadeBuilding{building})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	coordinates := make([]any, len(ring))
	for i, p := range ring {
		coordinates[i] = []any{p.X, p.Y}
	}

	height := 9.0
	features := make([]modelgeojson.Feature, 0, 1+len(receivers))
	features = append(features, modelgeojson.Feature{
		ID: "b1", Kind: modelgeojson.FeatureKindBuilding, HeightM: &height,
		GeometryType: modelgeojson.GeometryTypePolygon, Coordinates: []any{coordinates},
	})

	for _, r := range receivers {
		var facade any

		raw, err := json.Marshal(r.Facade)
		if err == nil {
			err = json.Unmarshal(raw, &facade)
		}

		if err != nil {
			t.Fatalf("facade key through JSON: %v", err)
		}

		receiverHeight := r.HeightM
		features = append(features, modelgeojson.Feature{
			ID: r.ID, Kind: modelgeojson.FeatureKindReceiver, HeightM: &receiverHeight,
			GeometryType: modelgeojson.GeometryTypePoint, Coordinates: []any{r.Point.X, r.Point.Y},
			Properties: map[string]any{
				modelgeojson.PropertyPlacementRule:    r.PlacementRule,
				modelgeojson.PropertyFacade:           facade,
				modelgeojson.PropertyStorey:           float64(r.Storey),
				modelgeojson.PropertySlabHeightSource: r.SlabHeightSource,
				modelgeojson.PropertyReviewStatus:     geo.ReviewStatusConfirmed,
			},
		})
	}

	return modelgeojson.Model{ProjectCRS: "EPSG:25832", Features: features}
}

func movedFacadeReceivers(report modelgeojson.ValidationReport) []string {
	var moved []string

	for _, issue := range report.Warnings {
		if issue.Code == modelgeojson.CodeReceiverFacadeMoved {
			moved = append(moved, issue.FeatureID)
		}
	}

	return moved
}

func modelSaveBody(t *testing.T, crs string, model json.RawMessage) string {
	t.Helper()

	body, err := json.Marshal(map[string]any{"crs": crs, "model": model})
	if err != nil {
		t.Fatalf("marshal save body: %v", err)
	}

	return string(body)
}

// storedModel reads the saved model back as a run does, in the project CRS,
// together with the bytes on disk.
func storedModel(t *testing.T, path string) (modelgeojson.Model, []byte) {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read normalized model: %v", err)
	}

	model, err := modelgeojson.Normalize(raw, "EPSG:25832", "test")
	if err != nil {
		t.Fatalf("normalize stored model: %v", err)
	}

	return model, raw
}

func assertNoMovedReceivers(t *testing.T, model modelgeojson.Model) {
	t.Helper()

	if moved := movedFacadeReceivers(modelgeojson.Validate(model)); len(moved) > 0 {
		t.Fatalf("receivers read as moved: %v", moved)
	}
}

func modelInWGS84(t *testing.T, handler http.Handler) json.RawMessage {
	t.Helper()

	rec := getModel(t, handler, "?crs=EPSG:4326")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response modelGetResponse

	decodeResponse(t, rec.Body.Bytes(), &response)

	return response.Model
}

// moveFirstReceiver shifts the first receiver of a GeoJSON model by dLon
// degrees and names it.
func moveFirstReceiver(t *testing.T, model json.RawMessage, dLon float64) (json.RawMessage, string) {
	t.Helper()

	var collection map[string]any

	err := json.Unmarshal(model, &collection)
	if err != nil {
		t.Fatalf("decode model: %v", err)
	}

	features, _ := collection["features"].([]any)
	for _, raw := range features {
		feature, _ := raw.(map[string]any)
		properties, _ := feature["properties"].(map[string]any)

		if properties["kind"] != modelgeojson.FeatureKindReceiver {
			continue
		}

		geometry, _ := feature["geometry"].(map[string]any)
		point, _ := geometry["coordinates"].([]any)
		lon, _ := point[0].(float64)
		point[0] = lon + dLon

		id, _ := feature["id"].(string)
		if id == "" {
			id, _ = properties["id"].(string)
		}

		edited, err := json.Marshal(collection)
		if err != nil {
			t.Fatalf("encode model: %v", err)
		}

		return edited, id
	}

	t.Fatal("the model has no receiver to move")

	return nil, ""
}

// The map's save path: the model goes out in EPSG:4326 and comes back in it.
// A round trip renumbers the facade keys of this building, and the save must
// not: it keeps the stored coordinates of every feature nobody moved, so no
// receiver reads as moved and the bytes on disk do not change. A receiver
// someone did move is saved where they put it, and keeps the endpoints of the
// building edge it still belongs to.
func TestModelSaveRoundTripKeepsFacadeKeys(t *testing.T) {
	t.Parallel()

	store := mustStore(t, "Model Keep Stored")
	handler := NewHandler(store, nil)
	path := store.ModelArtifactPaths().Normalized

	placed, err := json.Marshal(renumberingFacadeModel(t).ToFeatureCollection())
	if err != nil {
		t.Fatalf("marshal model: %v", err)
	}

	if rec := postModel(t, handler, modelSaveBody(t, "", placed)); rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	before, beforeBytes := storedModel(t, path)
	assertNoMovedReceivers(t, before)

	geographic := modelInWGS84(t, handler)

	if rec := postModel(t, handler, modelSaveBody(t, "EPSG:4326", geographic)); rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	after, afterBytes := storedModel(t, path)
	assertNoMovedReceivers(t, after)

	if string(afterBytes) != string(beforeBytes) {
		t.Fatal("a save of the unchanged model through EPSG:4326 must leave the stored model byte for byte as it was")
	}

	// About a metre east, at 52° N.
	edited, movedID := moveFirstReceiver(t, geographic, 1.5e-5)

	if rec := postModel(t, handler, modelSaveBody(t, "EPSG:4326", edited)); rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	final, _ := storedModel(t, path)

	for i, feature := range final.Features {
		previous := before.Features[i]

		if feature.ID != movedID {
			if !reflect.DeepEqual(feature, previous) {
				t.Errorf("feature %q was not moved, and the save changed it: %#v, was %#v", feature.ID, feature, previous)
			}

			continue
		}

		if reflect.DeepEqual(feature.Coordinates, previous.Coordinates) {
			t.Errorf("receiver %q was moved on the map, and the save kept its old point", movedID)
		}

		if !reflect.DeepEqual(feature.Properties[modelgeojson.PropertyFacade], previous.Properties[modelgeojson.PropertyFacade]) {
			t.Errorf("receiver %q kept its building edge, so its facade endpoints must be the stored ones", movedID)
		}
	}
}
