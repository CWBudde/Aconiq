package facadeset

import (
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

func customReceiver(id string, properties map[string]any) modelgeojson.Feature {
	height := 4.0

	return modelgeojson.Feature{
		ID: id, Kind: modelgeojson.FeatureKindReceiver, HeightM: &height,
		GeometryType: modelgeojson.GeometryTypePoint, Coordinates: []any{1.0, 2.0}, Properties: properties,
	}
}

func pointReceivers(features ...modelgeojson.Feature) []geo.PointReceiver {
	receivers := make([]geo.PointReceiver, len(features))
	for i, f := range features {
		receivers[i] = geo.PointReceiver{ID: f.ID, Point: geo.Point2D{X: 1, Y: 2}, HeightM: 4}
	}

	return receivers
}

// Custom keeps the model's receivers in their order, reads the facade
// attributes of those that carry any, and drops the rejected ones, counting
// them: a rejected point never enters a run.
func TestCustomDropsRejectedPointsAndReadsAttributes(t *testing.T) {
	t.Parallel()

	plain := customReceiver("plain", nil)
	proposed := customReceiver("proposed", map[string]any{
		modelgeojson.PropertyPlacementRule: geo.PlacementRLS19Facade,
		modelgeojson.PropertyReviewStatus:  geo.ReviewStatusProposed,
		modelgeojson.PropertyStorey:        float64(2),
	})
	rejected := customReceiver("rejected", map[string]any{modelgeojson.PropertyReviewStatus: geo.ReviewStatusRejected})
	model := modelgeojson.Model{Features: []modelgeojson.Feature{proposed, rejected, plain}}

	listed, dropped, err := Custom(pointReceivers(proposed, rejected, plain), model)
	if err != nil {
		t.Fatalf("Custom: %v", err)
	}

	if dropped != 1 {
		t.Errorf("rejected = %d, want 1", dropped)
	}

	if len(listed.Receivers) != 2 || listed.Receivers[0].ID != "proposed" || listed.Receivers[1].ID != "plain" {
		t.Fatalf("receivers = %+v, want proposed then plain", listed.Receivers)
	}

	if _, ok := listed.Facade["plain"]; ok {
		t.Error("a receiver with no facade property carries no attributes")
	}

	attrs := listed.Facade["proposed"]
	if attrs.PlacementRule != geo.PlacementRLS19Facade || attrs.Storey == nil || *attrs.Storey != 2 || !attrs.Proposed() {
		t.Errorf("attributes = %+v", attrs)
	}
}

func TestCustomRefusesASetOfRejectedPointsOnly(t *testing.T) {
	t.Parallel()

	rejected := customReceiver("r", map[string]any{modelgeojson.PropertyReviewStatus: geo.ReviewStatusRejected})

	_, _, err := Custom(pointReceivers(rejected), modelgeojson.Model{Features: []modelgeojson.Feature{rejected}})
	if err == nil || !strings.Contains(err.Error(), "every receiver in the model is rejected") {
		t.Fatalf("err = %v", err)
	}
}

func TestCustomRefusesAMalformedStorey(t *testing.T) {
	t.Parallel()

	odd := customReceiver("r", map[string]any{modelgeojson.PropertyStorey: 1.5})

	_, _, err := Custom(pointReceivers(odd), modelgeojson.Model{Features: []modelgeojson.Feature{odd}})
	if err == nil || !strings.Contains(err.Error(), `receiver "r"`) {
		t.Fatalf("err = %v", err)
	}
}

// A table row takes the facade attributes only as a complete set: rule, key
// and storey.
func TestAttributesComplete(t *testing.T) {
	t.Parallel()

	storey := 0
	key := &geo.FacadeKey{BuildingRef: "a"}

	for _, tc := range []struct {
		attrs Attributes
		want  bool
	}{
		{Attributes{PlacementRule: geo.PlacementRLS19Facade, Facade: key, Storey: &storey}, true},
		{Attributes{Facade: key, Storey: &storey}, false},
		{Attributes{PlacementRule: geo.PlacementRLS19Facade, Storey: &storey}, false},
		{Attributes{PlacementRule: geo.PlacementRLS19Facade, Facade: key}, false},
	} {
		if got := tc.attrs.Complete(); got != tc.want {
			t.Errorf("%+v: Complete() = %v, want %v", tc.attrs, got, tc.want)
		}
	}
}
