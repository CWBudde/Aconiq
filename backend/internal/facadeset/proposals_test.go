package facadeset

import (
	"reflect"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

const (
	photoHash = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	basis     = "engagement-2026-17-dpa-3"
)

// proposal is a window an agent read off a photo of the house's south
// facade: proposed, not sensitive, with the photo as its evidence.
func proposal(id string, uM float64) WindowPlacement {
	return WindowPlacement{
		ID: id, BuildingRef: "house",
		Frame: geo.FacadeWindow{UM: uM, SillZM: 3.9, WidthM: 1.2, HeightM: 1.5},
		Evidence: []any{map[string]any{
			"kind": "site-photo", "source_id": "photo-0001", "content_sha256": photoHash,
		}},
	}
}

// A proposal is added as a proposed, not sensitive window whose evidence
// carries the transmission decision it was made under; the windows a person
// already reviewed are left exactly as they were.
func TestProposeWindowsAddsProposals(t *testing.T) {
	t.Parallel()

	model := windowModel(t, windowSpec("bed", 2, true, geo.ReviewStatusConfirmed))
	before := model.Features[1]
	first := proposal("p1", 6)

	proposed, ids, err := ProposeWindows(model, []WindowPlacement{first, proposal("p2", 8)}, basis)
	if err != nil {
		t.Fatalf("ProposeWindows: %v", err)
	}

	if !reflect.DeepEqual(ids, []string{"p1", "p2"}) || len(proposed.Features) != 4 {
		t.Fatalf("ids = %v, features = %d, want p1 and p2 added to the model", ids, len(proposed.Features))
	}

	if !reflect.DeepEqual(proposed.Features[1], before) {
		t.Errorf("the confirmed window changed: %+v, was %+v", proposed.Features[1], before)
	}

	window, err := modelgeojson.WindowOf(proposed.Features[2])
	if err != nil {
		t.Fatalf("WindowOf: %v", err)
	}

	if window.ReviewStatus != geo.ReviewStatusProposed || window.Sensitive || window.Facade.BuildingRef != "house" {
		t.Errorf("p1 = %+v, want a proposed, not sensitive window on the house", window)
	}

	evidence, _ := proposed.Features[2].Properties[modelgeojson.PropertyEvidence].([]any)
	entry, _ := evidence[0].(map[string]any)

	if len(evidence) != 1 || entry[PropertyTransmissionBasis] != basis || entry["content_sha256"] != photoHash {
		t.Errorf("evidence = %#v, want the photo stamped with the transmission basis", evidence)
	}

	if _, stamped := first.Evidence[0].(map[string]any)[PropertyTransmissionBasis]; stamped {
		t.Error("the caller's evidence entry was stamped in place")
	}

	if report := modelgeojson.Validate(proposed); !report.Valid {
		t.Errorf("the proposed model does not validate: %+v", report.Errors)
	}
}

// What only a person may decide, a proposal may not claim, and a proposal
// must say what it was read from.
func TestProposeWindowsRefuses(t *testing.T) {
	t.Parallel()

	with := func(edit func(*WindowPlacement)) []WindowPlacement {
		p := proposal("p1", 6)
		edit(&p)

		return []WindowPlacement{p}
	}
	evidence := func(entry map[string]any) func(*WindowPlacement) {
		return func(p *WindowPlacement) { p.Evidence = []any{entry} }
	}

	cases := map[string]struct {
		proposals []WindowPlacement
		basis     string
		want      string
	}{
		"confirmed":        {with(func(p *WindowPlacement) { p.ReviewStatus = geo.ReviewStatusConfirmed }), basis, "person's act"},
		"rejected":         {with(func(p *WindowPlacement) { p.ReviewStatus = geo.ReviewStatusRejected }), basis, "person's act"},
		"sensitive":        {with(func(p *WindowPlacement) { p.Sensitive = true }), basis, "sensitive"},
		"no evidence":      {with(func(p *WindowPlacement) { p.Evidence = nil }), basis, "evidence is required"},
		"not an object":    {with(func(p *WindowPlacement) { p.Evidence = []any{"photo-0001"} }), basis, "must be an object"},
		"a google kind":    {with(evidence(map[string]any{"kind": "google", "source_id": "x", "content_sha256": photoHash})), basis, "kind"},
		"no source_id":     {with(evidence(map[string]any{"kind": "site-photo", "content_sha256": photoHash})), basis, "source_id"},
		"an uppercase sha": {with(evidence(map[string]any{"kind": "site-photo", "source_id": "x", "content_sha256": strings.ToUpper(photoHash)})), basis, "content_sha256"},
		"no hash":          {with(evidence(map[string]any{"kind": "plan", "source_id": "A-12"})), basis, "content_sha256"},
		"a path as source": {with(evidence(map[string]any{"kind": "site-photo", "source_id": "/Users/x/Musterstraße 1/IMG_0042.jpg", "content_sha256": photoHash})), basis, "never a path"},
		"a windows path":   {with(evidence(map[string]any{"kind": "site-photo", "source_id": `C:\photos\IMG_0042.jpg`, "content_sha256": photoHash})), basis, "never a path"},
		"an unknown key":   {with(evidence(map[string]any{"kind": "site-photo", "source_id": "x", "content_sha256": photoHash, "url": "https://x"})), basis, "evidence[0].url"},
		"its own basis":    {with(evidence(map[string]any{"kind": "site-photo", "source_id": "x", "content_sha256": photoHash, PropertyTransmissionBasis: "mine"})), basis, PropertyTransmissionBasis},
		"an existing id":   {with(func(p *WindowPlacement) { p.ID = "bed" }), basis, "already in the model"},
		"a building's id":  {with(func(p *WindowPlacement) { p.ID = "house" }), basis, "building feature"},
		"twice":            {[]WindowPlacement{proposal("p1", 6), proposal("p1", 8)}, basis, "given twice"},
		"past its facade":  {with(func(p *WindowPlacement) { p.Frame.UM = 9.5 }), basis, "runs past"},
		"no basis":         {[]WindowPlacement{proposal("p1", 6)}, " ", PropertyTransmissionBasis},
		"a basis path":     {[]WindowPlacement{proposal("p1", 6)}, "/clients/Musterstraße 1/dpa.pdf", "must not be a path"},
		"a pasted basis":   {[]WindowPlacement{proposal("p1", 6)}, strings.Repeat("x", 201), "longer than"},
		"nothing proposed": {nil, basis, ErrNoProposal.Error()},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			model := windowModel(t, windowSpec("bed", 2, true, geo.ReviewStatusConfirmed))

			_, _, err := ProposeWindows(model, tc.proposals, tc.basis)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want one mentioning %q", err, tc.want)
			}
		})
	}
}

// The window list's merge still replaces a window of the same id, as a
// corrected list imported again must; adding only never does.
func TestMergeWindowsReplacesOnlyWhenAsked(t *testing.T) {
	t.Parallel()

	model := windowModel(t, windowSpec("bed", 2, true, geo.ReviewStatusConfirmed))

	placer, err := modelgeojson.NewWindowPlacer(model)
	if err != nil {
		t.Fatalf("NewWindowPlacer: %v", err)
	}

	moved, err := placer.Feature(windowSpec("bed", 5, true, geo.ReviewStatusConfirmed))
	if err != nil {
		t.Fatalf("Feature: %v", err)
	}

	merged, replaced, err := MergeWindows(model, []modelgeojson.Feature{moved}, ReplaceWindows)
	if err != nil || replaced != 1 || len(merged.Features) != 2 || !reflect.DeepEqual(merged.Features[1], moved) {
		t.Fatalf("merged = %+v, replaced = %d, err = %v, want bed replaced in place", merged.Features, replaced, err)
	}

	if _, _, err := MergeWindows(model, []modelgeojson.Feature{moved}, AddWindowsOnly); err == nil {
		t.Error("AddWindowsOnly replaced an existing window")
	}
}
