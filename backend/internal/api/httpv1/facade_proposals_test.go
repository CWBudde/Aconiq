package httpv1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/facadeset"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/projectfs"
)

const (
	proposalPhotoHash = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	proposalBasis     = "engagement-2026-17/dpa-3"
)

// savedHouseWithABedroom saves elevationHouse with one confirmed, sensitive
// window on its south facade, and returns the handler, the store and the
// model's hash.
func savedHouseWithABedroom(t *testing.T) (http.Handler, projectfs.Store, string) {
	t.Helper()

	model, err := modelgeojson.Normalize([]byte(elevationHouse), "EPSG:25832", "test")
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	bed, err := facadeset.PlaceWindow(model, facadeset.WindowPlacement{
		ID: "bed", BuildingRef: "house",
		Frame:     geo.FacadeWindow{UM: 2, SillZM: 3.9, WidthM: 1.2, HeightM: 1.5},
		Sensitive: true, ReviewStatus: geo.ReviewStatusConfirmed,
	})
	if err != nil {
		t.Fatalf("PlaceWindow: %v", err)
	}

	var collection map[string]any

	err = json.Unmarshal([]byte(elevationHouse), &collection)
	if err != nil {
		t.Fatalf("decode house: %v", err)
	}

	features, _ := collection["features"].([]any)
	collection["features"] = append(features, bed)

	encoded, err := json.Marshal(map[string]any{"model": collection, "expected_hash": projectfs.ExpectNone})
	if err != nil {
		t.Fatalf("encode model: %v", err)
	}

	store := mustStore(t, "Window Proposals")
	handler := NewHandler(store, nil)

	return handler, store, savedModelHash(t, postModel(t, handler, string(encoded)))
}

func proposalWindow(id string) map[string]any {
	return map[string]any{
		"id": id, "building_ref": "house", "part": 0, "ring": 0, "edge": 0,
		"window":    map[string]any{"u_m": 6, "sill_z_m": 3.9, "width_m": 1.2, "height_m": 1.5},
		"sensitive": false,
		"evidence": []any{map[string]any{
			"kind": "site-photo", "source_id": "photo-0001", "content_sha256": proposalPhotoHash,
		}},
	}
}

func postProposal(t *testing.T, handler http.Handler, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encode proposal: %v", err)
	}

	req := newAPIRequest(http.MethodPost, "/api/v1/facades/windows", bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

// featureByID finds a feature of the stored model by its id, which the
// normalized file keeps among the properties.
func featureByID(t *testing.T, store projectfs.Store, id string) modelgeojson.GeoJSONFeature {
	t.Helper()

	for _, f := range readNormalizedModel(t, store).Features {
		if f.Properties["id"] == id {
			return f
		}
	}

	t.Fatalf("feature %q is not in the stored model", id)

	return modelgeojson.GeoJSONFeature{}
}

// A proposal is added to the saved model as a proposed, not sensitive window
// whose evidence names the photo and the transmission decision, and the
// window a person confirmed is left exactly as it was.
func TestFacadeWindowsProposeAddsWindows(t *testing.T) {
	t.Parallel()

	handler, store, hash := savedHouseWithABedroom(t)
	bed := featureByID(t, store, "bed")

	rec := postProposal(t, handler, map[string]any{
		"expected_hash": hash, "transmission_basis": proposalBasis,
		"windows": []any{proposalWindow("p1")},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp proposeWindowsResponse

	decodeResponse(t, rec.Body.Bytes(), &resp)

	if !reflect.DeepEqual(resp.WindowIDs, []string{"p1"}) || resp.Hash == "" || resp.Hash == hash {
		t.Errorf("response = %+v, want p1 and the new model's hash", resp)
	}

	if current, err := store.ModelHash(); err != nil || current != resp.Hash {
		t.Errorf("stored hash = %q (%v), want the receipt %q", current, err, resp.Hash)
	}

	if after := featureByID(t, store, "bed"); !reflect.DeepEqual(after, bed) {
		t.Errorf("the confirmed window changed:\n%+v\nwas\n%+v", after, bed)
	}

	p1 := featureByID(t, store, "p1")
	evidence, _ := p1.Properties["evidence"].([]any)

	var entry map[string]any
	if len(evidence) == 1 {
		entry, _ = evidence[0].(map[string]any)
	}

	if p1.Properties["review_status"] != geo.ReviewStatusProposed || p1.Properties["sensitive"] != false ||
		entry[facadeset.PropertyTransmissionBasis] != proposalBasis || entry["content_sha256"] != proposalPhotoHash {
		t.Errorf("p1 = %+v, want it proposed, not sensitive, with the photo and the basis as evidence", p1.Properties)
	}
}

// Every refusal answers before anything is written.
func TestFacadeWindowsProposeRefuses(t *testing.T) {
	t.Parallel()

	with := func(edit func(map[string]any)) []any {
		w := proposalWindow("p1")
		edit(w)

		return []any{w}
	}
	keep := func(map[string]any) {}

	// hash "current" sends the saved model's hash.
	cases := map[string]struct {
		hash   string
		basis  string
		window []any
		status int
		code   string
	}{
		"no expected_hash": {"", proposalBasis, with(keep), http.StatusBadRequest, errorCodeBadRequest},
		"none":             {projectfs.ExpectNone, proposalBasis, with(keep), http.StatusBadRequest, errorCodeBadRequest},
		"a stale hash":     {strings.Repeat("0", 64), proposalBasis, with(keep), http.StatusConflict, errorCodeHashMismatch},
		"no basis":         {"current", " ", with(keep), http.StatusBadRequest, errorCodeBadRequest},
		"confirmed":        {"current", proposalBasis, with(func(w map[string]any) { w["review_status"] = "confirmed" }), http.StatusBadRequest, errorCodeBadRequest},
		"sensitive":        {"current", proposalBasis, with(func(w map[string]any) { w["sensitive"] = true }), http.StatusBadRequest, errorCodeBadRequest},
		"no evidence":      {"current", proposalBasis, with(func(w map[string]any) { delete(w, "evidence") }), http.StatusBadRequest, errorCodeBadRequest},
		"the bedroom's id": {"current", proposalBasis, with(func(w map[string]any) { w["id"] = "bed" }), http.StatusBadRequest, errorCodeBadRequest},
		"no windows":       {"current", proposalBasis, []any{}, http.StatusBadRequest, errorCodeBadRequest},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, store, hash := savedHouseWithABedroom(t)

			expected := tc.hash
			if expected == "current" {
				expected = hash
			}

			before, err := os.ReadFile(store.ModelArtifactPaths().Normalized)
			if err != nil {
				t.Fatalf("read model: %v", err)
			}

			rec := postProposal(t, handler, map[string]any{
				"expected_hash": expected, "transmission_basis": tc.basis, "windows": tc.window,
			})
			assertErrorCode(t, rec, tc.status, tc.code)

			after, err := os.ReadFile(store.ModelArtifactPaths().Normalized)
			if err != nil {
				t.Fatalf("read model: %v", err)
			}

			if !bytes.Equal(before, after) {
				t.Error("a refused proposal changed the stored model")
			}
		})
	}
}

func TestFacadeWindowsProposeNeedsAModel(t *testing.T) {
	t.Parallel()

	rec := postProposal(t, NewHandler(mustStore(t, "No Model"), nil), map[string]any{
		"expected_hash": strings.Repeat("0", 64), "transmission_basis": proposalBasis,
		"windows": []any{proposalWindow("p1")},
	})
	assertErrorCode(t, rec, http.StatusNotFound, errorCodeModelNotFound)
}
