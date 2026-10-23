package wasmkernel_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/facadeset"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/wasmkernel"
)

// Two terraced houses in EPSG:25832, sharing a party wall at x = 547010, and
// two more that share one address. Synthetic: no real address.
const facadeTestModel = `{"type":"FeatureCollection","features":[
{"type":"Feature","id":"a","properties":{"kind":"building","height_m":9,"building:levels":"3",
 "addr:street":"Musterstraße","addr:housenumber":"1"},
 "geometry":{"type":"Polygon","coordinates":[[[547000,5795000],[547010,5795000],[547010,5795008],[547000,5795008],[547000,5795000]]]}},
{"type":"Feature","id":"b","properties":{"kind":"building","height_m":9,"building:levels":"3",
 "addr:street":"Musterstraße","addr:housenumber":"3"},
 "geometry":{"type":"Polygon","coordinates":[[[547010,5795000],[547020,5795000],[547020,5795008],[547010,5795008],[547010,5795000]]]}},
{"type":"Feature","id":"c","properties":{"kind":"building","height_m":6,
 "addr:street":"Doppelweg","addr:housenumber":"1"},
 "geometry":{"type":"Polygon","coordinates":[[[547100,5795000],[547110,5795000],[547110,5795008],[547100,5795008],[547100,5795000]]]}},
{"type":"Feature","id":"d","properties":{"kind":"building","height_m":6,
 "addr:street":"Doppelweg","addr:housenumber":"1"},
 "geometry":{"type":"Polygon","coordinates":[[[547200,5795000],[547210,5795000],[547210,5795008],[547200,5795008],[547200,5795000]]]}}
]}`

func facadeRequest(t *testing.T, buildings []string, allowUnconfirmed bool) []byte {
	t.Helper()

	in, err := json.Marshal(wasmkernel.FacadeReceiversRequest{
		CRS:              "EPSG:25832",
		Model:            json.RawMessage(facadeTestModel),
		StandardID:       "rls19-road",
		Buildings:        buildings,
		SpacingM:         4,
		AllowUnconfirmed: allowUnconfirmed,
	})
	if err != nil {
		t.Fatal(err)
	}

	return in
}

// The kernel adds a JSON boundary and nothing else: for the same model and
// request it places exactly the receivers facadeset places for `aconiq run`,
// with the same facade, storey and review attributes, and a building named by
// its address is the building named by its id.
func TestFacadeReceiversAnswersWhatTheCLIAnswers(t *testing.T) {
	t.Parallel()

	out, err := wasmkernel.FacadeReceivers(facadeRequest(t, []string{"Musterstraße 1"}, true))
	if err != nil {
		t.Fatalf("facadeReceivers: %v", err)
	}

	var resp wasmkernel.FacadeReceiversResponse

	err = json.Unmarshal(out, &resp)
	if err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	model, err := modelgeojson.Normalize([]byte(facadeTestModel), "EPSG:25832", "test")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	want, metadata, err := facadeset.Generate(facadeset.Request{Buildings: []string{"a"}, SpacingM: 4}, "rls19-road", model)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if len(resp.Receivers) != len(want.Receivers) || len(want.Receivers) == 0 {
		t.Fatalf("kernel placed %d receivers, facadeset %d", len(resp.Receivers), len(want.Receivers))
	}

	for i, got := range resp.Receivers {
		receiver := want.Receivers[i]
		attrs := want.Facade[receiver.ID]

		if got.PointReceiver != receiver {
			t.Fatalf("receiver %d = %+v, want %+v", i, got.PointReceiver, receiver)
		}

		key := attrs.Facade
		if got.PlacementRule != attrs.PlacementRule || got.Storey != *attrs.Storey ||
			got.SlabHeightSource != attrs.SlabHeightSource || got.ReviewStatus != attrs.ReviewStatus ||
			got.Facade.BuildingRef != key.BuildingRef || got.Facade.Part != key.Part || got.Facade.Ring != key.Ring ||
			got.Facade.Edge != key.Edge || got.Facade.From != [2]float64{key.From.X, key.From.Y} ||
			got.Facade.To != [2]float64{key.To.X, key.To.Y} {
			t.Fatalf("receiver %q = %+v, want %+v / %+v", got.ID, got, attrs, *key)
		}

		if got.Facade.BuildingRef != "a" {
			t.Fatalf("receiver %q stands on %q; only a was asked for", got.ID, got.Facade.BuildingRef)
		}
	}

	proposed, assumed := want.CountUnconfirmed()
	if resp.ProposedCount != proposed || resp.AssumedHeightCount != assumed || proposed != len(want.Receivers) {
		t.Fatalf("counts = %d proposed, %d assumed; want %d, %d", resp.ProposedCount, resp.AssumedHeightCount, proposed, assumed)
	}

	for key, value := range metadata {
		if resp.Metadata[key] != value {
			t.Errorf("metadata[%q] = %q, want %q", key, resp.Metadata[key], value)
		}
	}

	if resp.Metadata["allow_unconfirmed"] != "true" {
		t.Errorf("metadata = %v, want allow_unconfirmed recorded", resp.Metadata)
	}
}

// The kernel refuses what the run refuses, in the run's words: a normative
// standard over generated (proposed) points without the opt-in, an address two
// buildings share, a standard with no facade rule, and a request naming no
// building.
func TestFacadeReceiversRefusesWhatTheRunRefuses(t *testing.T) {
	t.Parallel()

	noRule, err := json.Marshal(wasmkernel.FacadeReceiversRequest{
		CRS: "EPSG:25832", Model: json.RawMessage(facadeTestModel), StandardID: "iso9613",
		Buildings: []string{"a"}, SpacingM: 4,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name  string
		in    []byte
		parts []string
	}{
		{"unconfirmed", facadeRequest(t, []string{"a"}, false), []string{"normative", "proposed facade point(s)", "--allow-unconfirmed"}},
		{"shared address", facadeRequest(t, []string{"Doppelweg 1"}, true), []string{"2 buildings", `"c"`, `"d"`}},
		{"no building", facadeRequest(t, nil, true), []string{"building"}},
		{"no rule", noRule, []string{"iso9613"}},
	} {
		_, err := wasmkernel.FacadeReceivers(tc.in)
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

// The facade a receiver answers with is the receiver table's own shape, so the
// browser writes it into a row as it arrives.
func TestFacadeReceiversSpeaksTheReceiverTableShape(t *testing.T) {
	t.Parallel()

	out, err := wasmkernel.FacadeReceivers(facadeRequest(t, []string{"b"}, true))
	if err != nil {
		t.Fatalf("facadeReceivers: %v", err)
	}

	var raw struct {
		Receivers []map[string]any `json:"receivers"`
	}

	err = json.Unmarshal(out, &raw)
	if err != nil {
		t.Fatal(err)
	}

	first := raw.Receivers[0]
	for _, key := range []string{"id", "point", "height_m", "placement_rule", "facade", "storey", "slab_height_source", "review_status"} {
		if _, ok := first[key]; !ok {
			t.Errorf("receiver %v has no %q", first, key)
		}
	}

	facade, _ := first["facade"].(map[string]any)
	if from, ok := facade["from"].([]any); !ok || len(from) != 2 || first["placement_rule"] != geo.PlacementRLS19Facade {
		t.Errorf("receiver %v: facade.from is not an [x, y] pair or the rule is not RLS-19's", first)
	}
}
