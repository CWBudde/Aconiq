package modelgeojson

import (
	"math"
	"slices"
	"testing"

	"github.com/aconiq/backend/internal/geo"
)

// renumberedFacadeModel is a round-tripped model from
// roundTrippedFacadeModel whose edges did renumber, together with the model
// as placed.
func renumberedFacadeModel(t *testing.T) (placed, renumbered Model) {
	t.Helper()

	for _, position := range renumberingPositions() {
		placed, roundTripped := roundTrippedFacadeModel(t, position[0], position[1])
		if len(movedReceiverIDs(Validate(roundTripped))) > 0 {
			return placed, roundTripped
		}
	}

	t.Fatal("no position renumbered its edges")

	return Model{}, Model{}
}

// withBuilding returns model with its building b1 replaced by footprint
// under id. The receivers keep their keys.
func withBuilding(model Model, id string, footprints ...[][][]geo.Point2D) Model {
	out := Model{ProjectCRS: model.ProjectCRS}

	for i, footprint := range footprints {
		buildingID := id
		if i > 0 {
			buildingID = id + "-copy"
		}

		out.Features = append(out.Features, buildingFeature(buildingID, footprint))
	}

	for _, feature := range model.Features {
		if feature.Kind != FeatureKindBuilding {
			out.Features = append(out.Features, feature)
		}
	}

	return out
}

func receiverIDs(model Model) []string {
	var ids []string

	for _, feature := range model.Features {
		if feature.Kind == FeatureKindReceiver {
			ids = append(ids, feature.ID)
		}
	}

	return ids
}

func reattachmentIDs(reattachments []FacadeReattachment, outcome string) []string {
	var ids []string

	for _, r := range reattachments {
		if r.Outcome == outcome {
			ids = append(ids, r.ReceiverID)
		}
	}

	return ids
}

func mustPropose(t *testing.T, model Model, maxShiftM float64) []FacadeReattachment {
	t.Helper()

	reattachments, err := ProposeFacadeReattachments(model, maxShiftM)
	if err != nil {
		t.Fatalf("ProposeFacadeReattachments: %v", err)
	}

	return reattachments
}

// assertReattached checks that applying every proposal leaves a model that
// validates clean, with each re-attached receiver proposed and otherwise as
// it was.
func assertReattached(t *testing.T, before Model, reattachments []FacadeReattachment) {
	t.Helper()

	after, applied := ApplyFacadeReattachments(before, reattachments)
	if want := len(reattachmentIDs(reattachments, ReattachProposed)); applied != want {
		t.Fatalf("applied %d, want %d", applied, want)
	}

	assertCleanReport(t, Validate(after))

	for i, feature := range after.Features {
		old := before.Features[i]
		if feature.ID != old.ID || feature.Kind != old.Kind {
			t.Fatalf("feature %d is %q (%s), was %q (%s)", i, feature.ID, feature.Kind, old.ID, old.Kind)
		}

		if feature.Kind != FeatureKindReceiver || !slices.Contains(reattachmentIDs(reattachments, ReattachProposed), feature.ID) {
			continue
		}

		if feature.Properties[PropertyReviewStatus] != geo.ReviewStatusProposed {
			t.Fatalf("%s: review_status = %v, want %s", feature.ID, feature.Properties[PropertyReviewStatus], geo.ReviewStatusProposed)
		}

		for _, unchanged := range []string{PropertyPlacementRule, PropertyStorey, PropertyStationUM, PropertySlabHeightSource} {
			if feature.Properties[unchanged] != old.Properties[unchanged] {
				t.Fatalf("%s: %s = %v, was %v; re-attaching changes the key only", feature.ID, unchanged,
					feature.Properties[unchanged], old.Properties[unchanged])
			}
		}

		if *feature.HeightM != *old.HeightM || !slices.Equal(feature.Coordinates.([]any), old.Coordinates.([]any)) {
			t.Fatalf("%s: position moved; a run never recomputes it from the key, and neither does re-attaching", feature.ID)
		}
	}
}

// A CRS round trip that renumbered a building's edges (B10) is repaired on
// the building itself, at the default 1 mm: every edge still runs where the
// key says, under another index.
func TestReattachFacadeAfterARenumbering(t *testing.T) {
	t.Parallel()

	_, renumbered := renumberedFacadeModel(t)

	reattachments := mustPropose(t, renumbered, FacadeMoveToleranceM)
	if got, want := reattachmentIDs(reattachments, ReattachProposed), receiverIDs(renumbered); !equalStrings(got, want) {
		t.Fatalf("proposed %v, want every receiver %v; reattachments %+v", got, want, reattachments)
	}

	for _, r := range reattachments {
		if r.Finding != CodeReceiverFacadeMoved || r.Proposed == nil || r.Proposed.BuildingRef != "b1" {
			t.Fatalf("%s: %+v, want a moved receiver proposed on b1", r.ReceiverID, r)
		}

		if r.Proposed.Edge == r.Previous.Edge || r.ShiftM == nil || *r.ShiftM > 1e-6 {
			t.Fatalf("%s: edge %d → %d, shift %v; want another index for the same edge", r.ReceiverID,
				r.Previous.Edge, r.Proposed.Edge, r.ShiftM)
		}
	}

	assertReattached(t, renumbered, reattachments)
}

// A building whose id changed but whose geometry did not, as when an LGLN
// import replaces the OSM copy of the same footprint, takes its receivers
// with it.
func TestReattachFacadeAfterAnIDChange(t *testing.T) {
	t.Parallel()

	model, _ := oneFacadeReceiver(t)
	model = withBuilding(model, "DENI03000000abcd", simpleFacadeFootprint())

	reattachments := mustPropose(t, model, FacadeMoveToleranceM)
	if len(reattachments) != 1 || reattachments[0].Outcome != ReattachProposed ||
		reattachments[0].Finding != CodeReceiverFacadeUnresolved || reattachments[0].Proposed.BuildingRef != "DENI03000000abcd" {
		t.Fatalf("reattachments = %+v, want the unresolved receiver proposed on the new id", reattachments)
	}

	if d := reattachments[0].ReceiverDistanceM; d == nil || math.Abs(*d-0.05) > 1e-9 {
		t.Fatalf("receiver distance = %v, want the RLS-19 0.05 m", d)
	}

	assertReattached(t, model, reattachments)
}

// The neighbour across a party wall has the same edge walked the other way.
// The match is direction-sensitive, so it never takes a receiver that stood
// on the other side of the wall.
func TestReattachFacadeNeverCrossesAPartyWall(t *testing.T) {
	t.Parallel()

	footprint := simpleFacadeFootprint()
	receivers := facadeReceiverFeatures(t, facadeTestBuilding(footprint), false)
	neighbour := [][][]geo.Point2D{{facadeRect(facadeX0+20, facadeY0, facadeX0+40, facadeY0+12)}}
	model := facadeModel(append([]Feature{buildingFeature("neighbour", neighbour)}, receivers...)...)

	reattachments := mustPropose(t, model, FacadeMoveToleranceM)
	if got := reattachmentIDs(reattachments, ReattachNoMatch); !equalStrings(got, receiverIDs(model)) {
		t.Fatalf("no-match %v, want every receiver; reattachments %+v", got, reattachments)
	}
}

// Two buildings with the edge the key names are two answers, and neither is
// guessed.
func TestReattachFacadeRefusesToChooseBetweenCopies(t *testing.T) {
	t.Parallel()

	model, _ := oneFacadeReceiver(t)
	model = withBuilding(model, "osm-way-1", simpleFacadeFootprint(), simpleFacadeFootprint())

	reattachments := mustPropose(t, model, FacadeMoveToleranceM)
	if len(reattachments) != 1 || reattachments[0].Outcome != ReattachAmbiguous || reattachments[0].Proposed != nil {
		t.Fatalf("reattachments = %+v, want one ambiguous without a proposal", reattachments)
	}

	refs := make([]string, 0, len(reattachments[0].Candidates))
	for _, candidate := range reattachments[0].Candidates {
		refs = append(refs, candidate.BuildingRef)
	}

	if !equalStrings(refs, []string{"osm-way-1", "osm-way-1-copy"}) {
		t.Fatalf("candidates = %v, want both copies in model order", refs)
	}

	after, applied := ApplyFacadeReattachments(model, reattachments)
	if applied != 0 || len(issuesWithCode(Validate(after).Warnings, CodeReceiverFacadeUnresolved)) != 1 {
		t.Fatalf("applied %d; an ambiguous receiver must stay as it was", applied)
	}
}

// A footprint edit beyond the tolerance is no match at 1 mm. Widening the
// tolerance is the engineer's call, and then the shift is reported.
func TestReattachFacadeToleranceIsTheEngineersCall(t *testing.T) {
	t.Parallel()

	model, _ := oneFacadeReceiver(t)
	model = withBuilding(model, "b1", [][][]geo.Point2D{{facadeRect(facadeX0+0.3, facadeY0, facadeX0+20.3, facadeY0+12)}})

	if got := mustPropose(t, model, FacadeMoveToleranceM); len(got) != 1 || got[0].Outcome != ReattachNoMatch {
		t.Fatalf("at 1 mm: %+v, want no match", got)
	}

	wide := mustPropose(t, model, 0.5)
	if len(wide) != 1 || wide[0].Outcome != ReattachProposed || wide[0].ShiftM == nil || math.Abs(*wide[0].ShiftM-0.3) > 1e-9 {
		t.Fatalf("at 0.5 m: %+v, want a proposal 0.3 m away", wide)
	}

	assertReattached(t, model, wide)
}

// On a geographic project the matching is in metres, in the compute CRS, and
// the new from / to are written back in degrees, so the model validates
// clean afterwards.
func TestReattachFacadeOnAGeographicProject(t *testing.T) {
	t.Parallel()

	model, _ := oneFacadeReceiver(t)
	model = withBuilding(model, "osm-way-2", simpleFacadeFootprint())

	geographic, err := Reproject(model, "EPSG:4326")
	if err != nil {
		t.Fatalf("Reproject: %v", err)
	}

	reattachments := mustPropose(t, geographic, FacadeMoveToleranceM)
	if len(reattachments) != 1 || reattachments[0].Outcome != ReattachProposed {
		t.Fatalf("reattachments = %+v, want one proposal", reattachments)
	}

	if from := reattachments[0].Proposed.From; math.Abs(from.X) > 180 || math.Abs(from.Y) > 90 {
		t.Fatalf("proposed from = %+v, want degrees, the project CRS", from)
	}

	assertReattached(t, geographic, reattachments)
}

// Where a geographic site has no metric CRS, nothing can be matched in
// metres, and each flagged receiver says so instead of being matched in
// degrees.
func TestReattachFacadeOnASiteWithNoComputeCRS(t *testing.T) {
	t.Parallel()

	reattachments := mustPropose(t, degreeFacadeModel(t, -100, 40), FacadeMoveToleranceM)
	if got := reattachmentIDs(reattachments, ReattachUnchecked); !equalStrings(got, []string{"r-keyed", "r-orphan"}) {
		t.Fatalf("unchecked %v, want r-keyed and r-orphan; reattachments %+v", got, reattachments)
	}
}

// Receivers validation does not flag are not touched, and a model with none
// has nothing to re-attach.
func TestReattachFacadeLeavesSoundReceiversAlone(t *testing.T) {
	t.Parallel()

	model, _ := oneFacadeReceiver(t)
	if got := mustPropose(t, model, FacadeMoveToleranceM); len(got) != 0 {
		t.Fatalf("reattachments = %+v, want none", got)
	}
}

func TestReattachFacadeRefusesABadTolerance(t *testing.T) {
	t.Parallel()

	model, _ := oneFacadeReceiver(t)

	for _, tol := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := ProposeFacadeReattachments(model, tol); err == nil {
			t.Fatalf("tolerance %v accepted", tol)
		}
	}
}
