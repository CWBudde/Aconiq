package sourcereview_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/sourcereview"
	"github.com/aconiq/backend/internal/standards/framework"
)

func source(id string, status any) modelgeojson.Feature {
	feature := modelgeojson.Feature{
		ID: id, Kind: modelgeojson.FeatureKindSource, SourceType: modelgeojson.SourceTypePoint,
		GeometryType: modelgeojson.GeometryTypePoint, Coordinates: []any{500000.0, 5800000.0},
	}
	if status != nil {
		feature.Properties = map[string]any{modelgeojson.PropertyReviewStatus: status}
	}

	return feature
}

func building(id string) modelgeojson.Feature {
	return modelgeojson.Feature{ID: id, Kind: modelgeojson.FeatureKindBuilding, GeometryType: modelgeojson.GeometryTypePolygon}
}

func model(features ...modelgeojson.Feature) modelgeojson.Model {
	return modelgeojson.Model{ProjectCRS: "EPSG:25832", Features: features}
}

var (
	normative = framework.ResolvedProfile{StandardID: "rls19-road", EvidenceTier: framework.EvidenceTierNormative}
	scaffold  = framework.ResolvedProfile{StandardID: "cnossos-road", EvidenceTier: framework.EvidenceTierScaffold}
)

func ids(m modelgeojson.Model) []string {
	out := make([]string, 0, len(m.Features))
	for _, f := range m.Features {
		out = append(out, f.ID)
	}

	return out
}

// A model nobody mapped from imagery runs as it always did.
func TestReviewLeavesAnUnreviewedModelAlone(t *testing.T) {
	t.Parallel()

	in := model(source("s1", nil), building("b1"), source("s2", "confirmed"))

	got, err := sourcereview.Review(in, normative, false)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(ids(got.Model), []string{"s1", "b1", "s2"}) || len(got.Excluded) != 0 || got.Proposed != 0 {
		t.Errorf("Review = %+v, want the model unchanged and nothing counted", got)
	}
}

// A rejected source is not there: every standard runs without it, and the
// rest of the model keeps its order.
func TestReviewLeavesRejectedSourcesOut(t *testing.T) {
	t.Parallel()

	in := model(source("s1", "rejected"), building("b1"), source("s2", nil), source("s3", "rejected"))

	for _, profile := range []framework.ResolvedProfile{normative, scaffold} {
		got, err := sourcereview.Review(in, profile, false)
		if err != nil {
			t.Fatalf("%s: %v", profile.StandardID, err)
		}

		if !slices.Equal(ids(got.Model), []string{"b1", "s2"}) || !slices.Equal(got.Excluded, []string{"s1", "s3"}) {
			t.Errorf("%s: Review = %+v, want s1 and s3 left out", profile.StandardID, got)
		}
	}

	if len(in.Features) != 4 {
		t.Errorf("Review changed its input: %v", ids(in))
	}
}

// A normative run over a source nobody confirmed is refused, as one over an
// unconfirmed facade point is, and the same opt-in lifts it.
func TestReviewRefusesANormativeRunOverProposedSources(t *testing.T) {
	t.Parallel()

	in := model(source("s1", "proposed"), source("s2", "proposed"), source("s3", "rejected"))

	_, err := sourcereview.Review(in, normative, false)
	if err == nil {
		t.Fatal("want a refusal of a normative run over two proposed sources")
	}

	for _, want := range []string{`"rls19-road" is normative`, "2 proposed source(s)", "review_status: confirmed", "--allow-unconfirmed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not say %q", err, want)
		}
	}

	got, err := sourcereview.Review(in, normative, true)
	if err != nil {
		t.Fatalf("with the opt-in: %v", err)
	}

	if got.Proposed != 2 || !slices.Equal(ids(got.Model), []string{"s1", "s2"}) {
		t.Errorf("with the opt-in: Review = %+v, want both proposed sources computed and counted", got)
	}
}

// A standard that assesses nothing runs over proposed sources, and still
// counts them.
func TestReviewCountsProposedSourcesOfANonNormativeRun(t *testing.T) {
	t.Parallel()

	got, err := sourcereview.Review(model(source("s1", "proposed")), scaffold, false)
	if err != nil || got.Proposed != 1 {
		t.Errorf("Review = %+v, %v, want one proposed source counted and no refusal", got, err)
	}
}

// The browser reaches Review through the kernel with a model nothing has
// validated yet, so a status it does not know is refused, not computed.
func TestReviewRefusesAnUnknownStatus(t *testing.T) {
	t.Parallel()

	for _, status := range []any{"maybe", true} {
		_, err := sourcereview.Review(model(source("s1", status)), scaffold, true)
		if err == nil || !strings.Contains(err.Error(), `source "s1"`) {
			t.Errorf("status %#v: err = %v, want a refusal naming s1", status, err)
		}
	}
}
