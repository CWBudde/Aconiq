package wasmkernel_test

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/wasmkernel"
)

const reviewedSourcesModel = `{"type":"FeatureCollection","features":[
  {"type":"Feature","properties":{"id":"traced","kind":"source","source_type":"area","review_status":"proposed"},
   "geometry":{"type":"Polygon","coordinates":[[[0,0],[20,0],[20,10],[0,0]]]}},
  {"type":"Feature","properties":{"id":"gone","kind":"source","source_type":"line","review_status":"rejected"},
   "geometry":{"type":"LineString","coordinates":[[0,-20],[100,-20]]}},
  {"type":"Feature","properties":{"id":"planned","kind":"source","source_type":"line"},
   "geometry":{"type":"LineString","coordinates":[[0,-40],[100,-40]]}}]}`

func reviewSources(t *testing.T, allow bool) (wasmkernel.ReviewSourcesResponse, error) {
	t.Helper()

	request, err := json.Marshal(wasmkernel.ReviewSourcesRequest{
		CRS: "EPSG:25832", Model: json.RawMessage(reviewedSourcesModel), StandardID: "rls19-road", AllowUnconfirmed: allow,
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := wasmkernel.ReviewSources(request)
	if err != nil {
		return wasmkernel.ReviewSourcesResponse{}, err
	}

	var resp wasmkernel.ReviewSourcesResponse

	err = json.Unmarshal(out, &resp)
	if err != nil {
		t.Fatalf("decode %s: %v", out, err)
	}

	return resp, nil
}

// The browser asks the kernel which sources a run computes, so it refuses a
// normative run over a proposed source in `aconiq run`'s words, and leaves
// out the rejected ones the CLI leaves out.
func TestReviewSourcesAnswersAsTheRunDoes(t *testing.T) {
	t.Parallel()

	_, err := reviewSources(t, false)
	if err == nil || !strings.Contains(err.Error(), "1 proposed source(s)") || !strings.Contains(err.Error(), "--allow-unconfirmed") {
		t.Fatalf("err = %v, want the run's refusal of one proposed source", err)
	}

	resp, err := reviewSources(t, true)
	if err != nil {
		t.Fatal(err)
	}

	wantMetadata := map[string]string{"source_proposed_count": "1", "source_rejected_count": "1", "allow_unconfirmed": "true"}
	if !slices.Equal(resp.ExcludedIDs, []string{"gone"}) || resp.ProposedCount != 1 || resp.RejectedCount != 1 ||
		!maps.Equal(resp.Metadata, wantMetadata) {
		t.Errorf("reviewSources = %+v, want gone excluded, one proposed and the CLI's metadata", resp)
	}
}

func TestReviewSourcesNeedsItsRequest(t *testing.T) {
	t.Parallel()

	for name, input := range map[string]string{
		"not json": `{`,
		"no crs":   `{"model":` + reviewedSourcesModel + `,"standard_id":"rls19-road"}`,
		"unknown":  `{"crs":"EPSG:25832","model":` + reviewedSourcesModel + `,"standard_id":"nope"}`,
	} {
		if _, err := wasmkernel.ReviewSources([]byte(input)); err == nil {
			t.Errorf("%s: want a refusal", name)
		}
	}
}
