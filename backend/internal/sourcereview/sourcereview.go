// Package sourcereview settles which of a model's sources a run computes:
// a source mapped from imagery carries a review_status (P18 E2), and a run
// must not quietly assess over one nobody confirmed.
//
// It is the one implementation of that rule. `aconiq run` and its estimate
// call it on the model they load, and the WASM kernel's `reviewSources`
// answers the browser from it, so both targets refuse in the same words and
// leave out the same sources.
package sourcereview

import (
	"fmt"
	"slices"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/standards/framework"
)

const op = "run.sources"

// Result is the model a run computes and what reviewing its sources found.
type Result struct {
	// Model is the input without its rejected sources, every other feature
	// in its original order. The input is not changed.
	Model modelgeojson.Model

	// Excluded names the rejected sources, in model order.
	Excluded []string

	// Proposed counts the sources the run computes although nobody has
	// confirmed them; only a run that opted in, or a non-normative one, gets
	// this far with any.
	Proposed int
}

// Review leaves a model's rejected sources out and refuses a normative run
// over proposed ones unless it opted in (allowUnconfirmed, the
// `--allow-unconfirmed` that lifts the facade-receiver gate too). A source
// without a review_status was taken from plans or an import and is computed
// as it always was; so is a confirmed one. A non-normative standard assesses
// nothing, so it runs over proposed sources and only counts them.
//
// A review_status Review does not know is refused rather than read as
// absent: the kernel hands it models nothing has validated.
func Review(model modelgeojson.Model, standard framework.ResolvedProfile, allowUnconfirmed bool) (Result, error) {
	result := Result{Model: model}
	result.Model.Features = make([]modelgeojson.Feature, 0, len(model.Features))

	for _, feature := range model.Features {
		status, err := reviewStatus(feature)
		if err != nil {
			return Result{}, err
		}

		switch status {
		case geo.ReviewStatusRejected:
			result.Excluded = append(result.Excluded, feature.ID)

			continue
		case geo.ReviewStatusProposed:
			result.Proposed++
		}

		result.Model.Features = append(result.Model.Features, feature)
	}

	if result.Proposed == 0 || allowUnconfirmed || standard.EvidenceTier != framework.EvidenceTierNormative {
		return result, nil
	}

	return Result{}, domainerrors.New(domainerrors.KindUserInput, op, fmt.Sprintf(
		"standard %q is normative and the model includes %d proposed source(s): "+
			"nobody has confirmed these sources, so levels computed from them are not an assessment; "+
			"confirm them in the model (review_status: confirmed), or pass --allow-unconfirmed to run anyway and have the count recorded with the result",
		standard.StandardID, result.Proposed,
	), nil)
}

var statuses = []string{geo.ReviewStatusProposed, geo.ReviewStatusConfirmed, geo.ReviewStatusRejected}

// reviewStatus is a source's review_status, "" for a feature that is not a
// source or carries none.
func reviewStatus(feature modelgeojson.Feature) (string, error) {
	if !strings.EqualFold(strings.TrimSpace(feature.Kind), modelgeojson.FeatureKindSource) {
		return "", nil
	}

	value, present := feature.Properties[modelgeojson.PropertyReviewStatus]
	if !present || value == nil {
		return "", nil
	}

	status, isString := value.(string)
	if !isString || !slices.Contains(statuses, status) {
		return "", domainerrors.New(domainerrors.KindUserInput, op, fmt.Sprintf(
			"source %q: review_status %#v is not one of %s", feature.ID, value, strings.Join(statuses, "|"),
		), nil)
	}

	return status, nil
}
