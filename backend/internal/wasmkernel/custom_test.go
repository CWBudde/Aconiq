package wasmkernel_test

import (
	"encoding/json"
	"maps"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/facadeset"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/wasmkernel"
)

// customTestModel is facadeTestModel's buildings plus hand-placed receivers:
// the first three facade points facadeset places on building a, confirmed,
// proposed and rejected, and a plain receiver with no facade link. edit, when
// set, rewrites the confirmed receiver's properties.
func customTestModel(t *testing.T, edit func(map[string]any)) (json.RawMessage, []geo.PointReceiver) {
	t.Helper()

	model, err := modelgeojson.Normalize([]byte(facadeTestModel), "EPSG:25832", "test")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	placed, _, err := facadeset.Generate(facadeset.Request{Buildings: []string{"a"}, SpacingM: 4}, "rls19-road", model)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	var receivers []geo.PointReceiver

	add := func(receiver geo.PointReceiver, properties map[string]any) {
		height := receiver.HeightM
		model.Features = append(model.Features, modelgeojson.Feature{
			ID: receiver.ID, Kind: modelgeojson.FeatureKindReceiver, HeightM: &height,
			GeometryType: modelgeojson.GeometryTypePoint, Coordinates: []any{receiver.Point.X, receiver.Point.Y},
			Properties: properties,
		})
		receivers = append(receivers, receiver)
	}

	for i, status := range []string{geo.ReviewStatusConfirmed, geo.ReviewStatusProposed, geo.ReviewStatusRejected} {
		receiver := placed.Receivers[i]
		attrs := placed.Facade[receiver.ID]

		key, err := json.Marshal(attrs.Facade)
		if err != nil {
			t.Fatal(err)
		}

		var facade map[string]any

		err = json.Unmarshal(key, &facade)
		if err != nil {
			t.Fatal(err)
		}

		properties := map[string]any{
			modelgeojson.PropertyPlacementRule:    attrs.PlacementRule,
			modelgeojson.PropertyFacade:           facade,
			modelgeojson.PropertyStorey:           float64(*attrs.Storey),
			modelgeojson.PropertySlabHeightSource: geo.SlabHeightLevels,
			modelgeojson.PropertyReviewStatus:     status,
		}

		if i == 0 && edit != nil {
			edit(properties)
		}

		add(receiver, properties)
	}

	add(geo.PointReceiver{ID: "plain", Point: geo.Point2D{X: 547050, Y: 5795050}, HeightM: 4}, nil)

	out, err := json.Marshal(model.ToFeatureCollection())
	if err != nil {
		t.Fatal(err)
	}

	return out, receivers
}

func customRequest(t *testing.T, model json.RawMessage, receivers []geo.PointReceiver, allowUnconfirmed bool) []byte {
	t.Helper()

	in, err := json.Marshal(wasmkernel.CustomReceiversRequest{
		CRS: "EPSG:25832", Model: model, StandardID: "rls19-road",
		Receivers: receivers, AllowUnconfirmed: allowUnconfirmed,
	})
	if err != nil {
		t.Fatal(err)
	}

	return in
}

// The kernel adds a JSON boundary to facadeset.Custom and Gate, the calls
// `aconiq run --receiver-mode custom` makes: it keeps the same receivers in
// the same order, drops the rejected one, counts what the run counts, and
// hands the table attributes only to a receiver carrying the complete set.
func TestCustomReceiversAnswersWhatTheCLIAnswers(t *testing.T) {
	t.Parallel()

	raw, receivers := customTestModel(t, nil)

	out, err := wasmkernel.CustomReceivers(customRequest(t, raw, receivers, true))
	if err != nil {
		t.Fatalf("customReceivers: %v", err)
	}

	var resp wasmkernel.CustomReceiversResponse

	err = json.Unmarshal(out, &resp)
	if err != nil {
		t.Fatal(err)
	}

	model, err := modelgeojson.Normalize(raw, "EPSG:25832", "test")
	if err != nil {
		t.Fatal(err)
	}

	want, rejected, err := facadeset.Custom(receivers, model)
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.Receivers) != len(want.Receivers) || len(want.Receivers) != 3 {
		t.Fatalf("kernel kept %d receivers, facadeset %d; want 3", len(resp.Receivers), len(want.Receivers))
	}

	for i, got := range resp.Receivers {
		if got.PointReceiver != want.Receivers[i] {
			t.Fatalf("receiver %d = %+v, want %+v", i, got.PointReceiver, want.Receivers[i])
		}

		attrs, carries := want.Facade[got.ID]
		if !carries || !attrs.Complete() {
			if got.Facade != nil || got.Storey != nil || got.PlacementRule != "" {
				t.Errorf("receiver %q carries table attributes it has no complete set of: %+v", got.ID, got)
			}

			continue
		}

		if got.Facade == nil || got.Storey == nil || *got.Storey != *attrs.Storey || got.PlacementRule != attrs.PlacementRule ||
			got.Facade.Edge != attrs.Facade.Edge || got.Facade.From != [2]float64{attrs.Facade.From.X, attrs.Facade.From.Y} {
			t.Errorf("receiver %q = %+v, want %+v", got.ID, got, attrs)
		}
	}

	proposed, assumed := want.CountUnconfirmed()
	if resp.ProposedCount != proposed || resp.AssumedHeightCount != assumed || resp.RejectedCount != rejected ||
		proposed != 1 || rejected != 1 {
		t.Fatalf("counts = %d proposed, %d assumed, %d rejected; want %d, %d, %d",
			resp.ProposedCount, resp.AssumedHeightCount, resp.RejectedCount, proposed, assumed, rejected)
	}

	if resp.Metadata["allow_unconfirmed"] != "true" {
		t.Errorf("metadata = %v, want allow_unconfirmed recorded", resp.Metadata)
	}
}

// The kernel refuses what the run refuses, in its words: a proposed point
// without the opt-in, a broken facade link and another standard's rule even
// with it, and a model whose every receiver is rejected.
func TestCustomReceiversRefusesWhatTheRunRefuses(t *testing.T) {
	t.Parallel()

	moved := func(p map[string]any) {
		facade := maps.Clone(p[modelgeojson.PropertyFacade].(map[string]any))
		facade["from"] = []any{0.0, 0.0}
		p[modelgeojson.PropertyFacade] = facade
	}
	foreign := func(p map[string]any) { p[modelgeojson.PropertyPlacementRule] = geo.PlacementSchall03Facade }

	allRejected, err := json.Marshal(map[string]any{"type": "FeatureCollection", "features": []any{map[string]any{
		"type": "Feature", "id": "r",
		"properties": map[string]any{"kind": "receiver", "height_m": 4, "review_status": "rejected"},
		"geometry":   map[string]any{"type": "Point", "coordinates": []any{547050, 5795050}},
	}}})
	if err != nil {
		t.Fatal(err)
	}

	only := []geo.PointReceiver{{ID: "r", Point: geo.Point2D{X: 547050, Y: 5795050}, HeightM: 4}}

	unconfirmedModel, unconfirmedReceivers := customTestModel(t, nil)
	movedModel, movedReceivers := customTestModel(t, moved)
	foreignModel, foreignReceivers := customTestModel(t, foreign)

	for _, tc := range []struct {
		name  string
		in    []byte
		parts []string
	}{
		{"unconfirmed", customRequest(t, unconfirmedModel, unconfirmedReceivers, false), []string{"normative", "1 proposed facade point(s)", "--allow-unconfirmed"}},
		{"moved", customRequest(t, movedModel, movedReceivers, true), []string{modelgeojson.CodeReceiverFacadeMoved, "does not apply"}},
		{"foreign rule", customRequest(t, foreignModel, foreignReceivers, true), []string{geo.PlacementSchall03Facade, "rls19-road"}},
		{"all rejected", customRequest(t, allRejected, only, true), []string{"every receiver in the model is rejected"}},
	} {
		_, err := wasmkernel.CustomReceivers(tc.in)
		if err == nil {
			t.Errorf("%s: no refusal", tc.name)

			continue
		}

		for _, part := range tc.parts {
			if !strings.Contains(err.Error(), part) {
				t.Errorf("%s: refusal %q does not contain %q", tc.name, err, part)
			}
		}
	}
}
