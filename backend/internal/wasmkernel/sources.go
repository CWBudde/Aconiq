package wasmkernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/sourcereview"
)

// ReviewSourcesRequest is the JSON `aconiq.reviewSources` takes: the run's
// sources as a v1 model in CRS, the standard the run computes and
// `--allow-unconfirmed`. The model may hold other kinds too; only sources are
// reviewed.
type ReviewSourcesRequest struct {
	CRS              string          `json:"crs"`
	Model            json.RawMessage `json:"model"`
	StandardID       string          `json:"standard_id"`
	AllowUnconfirmed bool            `json:"allow_unconfirmed,omitempty"`
}

// ReviewSourcesResponse names the rejected sources a run leaves out, counts
// the proposed ones it computes, and carries the metadata `aconiq run`
// records for both — the keys cli.receiverSet.withSources writes.
type ReviewSourcesResponse struct {
	ExcludedIDs   []string          `json:"excluded_ids"`
	ProposedCount int               `json:"proposed_count"`
	RejectedCount int               `json:"rejected_count"`
	Metadata      map[string]string `json:"metadata"`
}

// ReviewSources settles which sources a browser-mode run computes through
// sourcereview.Review, the call `aconiq run` makes on the model it loads: a
// rejected source is left out, and a normative run over a proposed one is
// refused in the run's words unless it opted in. The browser filters its
// sources by ExcludedIDs and never decides this itself.
func ReviewSources(input []byte) ([]byte, error) {
	var req ReviewSourcesRequest

	err := json.Unmarshal(input, &req)
	if err != nil {
		return nil, fmt.Errorf("invalid input JSON: %w", err)
	}

	if req.CRS == "" {
		return nil, errors.New("crs is required: the compute CRS the model's coordinates are in")
	}

	model, err := modelgeojson.Normalize(req.Model, req.CRS, "reviewSources")
	if err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}

	standard, err := resolveStandard(req.StandardID)
	if err != nil {
		return nil, err
	}

	review, err := sourcereview.Review(model, standard, req.AllowUnconfirmed)
	if err != nil {
		return nil, fmt.Errorf("sources: %w", err)
	}

	resp := ReviewSourcesResponse{
		ExcludedIDs:   append([]string{}, review.Excluded...),
		ProposedCount: review.Proposed,
		RejectedCount: len(review.Excluded),
		Metadata:      map[string]string{},
	}

	if resp.ProposedCount > 0 {
		resp.Metadata["source_proposed_count"] = strconv.Itoa(resp.ProposedCount)

		if req.AllowUnconfirmed {
			resp.Metadata[allowUnconfirmedKey] = metadataTrue
		}
	}

	if resp.RejectedCount > 0 {
		resp.Metadata["source_rejected_count"] = strconv.Itoa(resp.RejectedCount)
	}

	out, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	return out, nil
}

// The provenance key a run that opted in records, and its value: the
// metadata `aconiq run --allow-unconfirmed` writes, which the custom, facade
// and source answers all carry.
const (
	allowUnconfirmedKey = "allow_unconfirmed"
	metadataTrue        = "true"
)
