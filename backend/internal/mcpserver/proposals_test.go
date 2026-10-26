package mcpserver_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/mcpserver"
)

const proposalPhotoHash = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

// houseWithABedroom is a 10 m × 8 m house and one window on its south
// facade that a person has confirmed and marked sensitive.
const houseWithABedroom = `{
  "type": "FeatureCollection",
  "features": [
    {"type": "Feature", "id": "house", "properties": {"kind": "building", "height_m": 9},
     "geometry": {"type": "Polygon", "coordinates": [[[547000, 5795000], [547010, 5795000], [547010, 5795008], [547000, 5795008], [547000, 5795000]]]}},
    {"type": "Feature", "id": "bed", "properties": {"kind": "window",
      "facade": {"building_ref": "house", "part": 0, "ring": 0, "edge": 0, "from": [547000, 5795000], "to": [547010, 5795000]},
      "window": {"u_m": 2, "sill_z_m": 3.9, "width_m": 1.2, "height_m": 1.5},
      "sensitive": true, "review_status": "confirmed"},
     "geometry": {"type": "Point", "coordinates": [547002.6, 5795000]}}
  ]
}`

// proposedWindow is a window an agent read off a photo of the south facade.
func proposedWindow(id string) map[string]any {
	return map[string]any{
		"id": id, "building_ref": "house", "part": 0, "ring": 0, "edge": 0,
		"window": map[string]any{"u_m": 6, "sill_z_m": 3.9, "width_m": 1.2, "height_m": 1.5},
		"evidence": []any{map[string]any{
			"kind": "site-photo", "source_id": "photo-0001", "content_sha256": proposalPhotoHash,
		}},
	}
}

func proposeArgs(window map[string]any, confirm string) map[string]any {
	return map[string]any{"windows": []any{window}, "confirm": confirm}
}

// savedHouse saves houseWithABedroom through save_model and returns its hash.
func savedHouse(t *testing.T, f fixture) string {
	t.Helper()

	var saved struct {
		Hash string `json:"hash"`
	}

	result := f.call(t, "save_model", saveArgs(houseWithABedroom, "none"))
	if result.IsError {
		t.Fatalf("save_model: %s", resultText(t, result))
	}

	err := json.Unmarshal([]byte(resultText(t, result)), &saved)
	if err != nil || saved.Hash == "" {
		t.Fatalf("save_model answered %s", resultText(t, result))
	}

	return saved.Hash
}

// windowsOf reads the saved model's windows by id, through get_model.
func windowsOf(t *testing.T, f fixture) map[string]map[string]any {
	t.Helper()

	var answer struct {
		Model struct {
			Features []struct {
				Properties map[string]any `json:"properties"`
			} `json:"features"`
		} `json:"model"`
	}

	err := json.Unmarshal([]byte(resultText(t, f.call(t, "get_model", map[string]any{}))), &answer)
	if err != nil {
		t.Fatalf("decode get_model: %v", err)
	}

	windows := make(map[string]map[string]any)

	for _, feature := range answer.Model.Features {
		if feature.Properties["kind"] == "window" {
			id, _ := feature.Properties["id"].(string)
			windows[id] = feature.Properties
		}
	}

	return windows
}

// propose_windows is offered only by a server started with writes allowed
// and a photo transmission decision, which only the person starting it can
// record; New refuses a decision without writes, and one spelled as a path.
func TestProposeWindowsNeedsAPhotoTransmissionDecision(t *testing.T) {
	t.Parallel()

	writesOnly := newFixtureWith(t, mcpserver.Options{AllowWrites: true}, nil)
	if _, listed := listTools(t, writesOnly.session)["propose_windows"]; listed {
		t.Error("a server started without a photo transmission decision offers propose_windows")
	}

	for name, opts := range map[string]mcpserver.Options{
		"without writes":  {PhotoTransmission: testPhotoTransmission},
		"a path":          {AllowWrites: true, PhotoTransmission: "/clients/Musterstraße 1/dpa.pdf"},
		"a windows path":  {AllowWrites: true, PhotoTransmission: `C:\engagements\dpa.pdf`},
		"a pasted record": {AllowWrites: true, PhotoTransmission: strings.Repeat("x", 201)},
	} {
		opts.AuditPath = filepath.Join(t.TempDir(), "mcp-audit.jsonl")

		if _, err := mcpserver.New(http.NotFoundHandler(), opts); err == nil {
			t.Errorf("%s: New accepted the photo transmission decision", name)
		}
	}
}

// A proposal lands in the model as a proposed, not sensitive window whose
// evidence names the photo and the decision; the window a person confirmed
// is untouched; and the audit line records the model it left and the
// decision it was made under.
func TestProposeWindowsAddsProposalsAndAuditsTheDecision(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	hash := savedHouse(t, f)
	bed := windowsOf(t, f)["bed"]

	result := f.call(t, "propose_windows", proposeArgs(proposedWindow("p1"), hash))
	if result.IsError {
		t.Fatalf("propose_windows: %s", resultText(t, result))
	}

	var answer struct {
		Hash      string   `json:"hash"`
		WindowIDs []string `json:"window_ids"`
	}

	err := json.Unmarshal([]byte(resultText(t, result)), &answer)
	if err != nil || !slices.Equal(answer.WindowIDs, []string{"p1"}) || answer.Hash == hash {
		t.Fatalf("propose_windows answered %s", resultText(t, result))
	}

	windows := windowsOf(t, f)

	if !maps.EqualFunc(windows["bed"], bed, jsonEqual) {
		t.Errorf("the confirmed window changed: %v, was %v", windows["bed"], bed)
	}

	p1 := windows["p1"]
	evidence, _ := p1["evidence"].([]any)

	var entry map[string]any
	if len(evidence) == 1 {
		entry, _ = evidence[0].(map[string]any)
	}

	if p1["review_status"] != "proposed" || p1["sensitive"] != false || entry["transmission_basis"] != testPhotoTransmission {
		t.Errorf("p1 = %v, want it proposed, not sensitive, stamped with the decision", p1)
	}

	lines := auditLines(t, f.store)
	last := lines[len(lines)-1]

	if last["tool"] != "propose_windows" || last["outcome"] != "ok" || last["model_hash"] != answer.Hash ||
		last["photo_transmission"] != testPhotoTransmission {
		t.Errorf("audit line = %v, want propose_windows ok, the new model and the decision", last)
	}

	// A proposal against the model read before the first one landed is
	// refused, and audited with the decision.
	stale := f.call(t, "propose_windows", proposeArgs(proposedWindow("p2"), hash))
	if apiErr := apiErrorOf(t, stale); apiErr["code"] != "hash_mismatch" {
		t.Errorf("a stale confirm answered %v, want hash_mismatch", apiErr)
	}

	lines = auditLines(t, f.store)
	if last := lines[len(lines)-1]; last["outcome"] != "hash_mismatch" || last["photo_transmission"] != testPhotoTransmission {
		t.Errorf("audit line = %v, want the refusal recorded with the decision", last)
	}

	if _, added := windowsOf(t, f)["p2"]; added {
		t.Error("a refused proposal was written")
	}
}

func jsonEqual(a, b any) bool {
	left, errA := json.Marshal(a)
	right, errB := json.Marshal(b)

	return errA == nil && errB == nil && string(left) == string(right)
}
