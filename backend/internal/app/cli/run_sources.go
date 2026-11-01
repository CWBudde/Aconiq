package cli

import (
	"fmt"
	"strconv"

	"github.com/aconiq/backend/internal/sourcereview"
	"github.com/aconiq/backend/internal/standards/framework"
)

// The keys a run over reviewed sources adds to provenance metadata and the
// run summary (P18 E2). Each is written only when it is non-zero, so a model
// nobody mapped from imagery records what it always did.
const (
	sourceProposedCountKey = "source_proposed_count"
	sourceRejectedCountKey = "source_rejected_count"
)

// reviewRunSources leaves the model's rejected sources out and refuses a
// normative run over proposed ones without --allow-unconfirmed, through
// sourcereview.Review — the call the WASM kernel's reviewSources makes for a
// browser run. The run and its estimate both call it, before the receivers,
// so both refuse in the same words and before a run exists.
func reviewRunSources(inputs runInputs, standard framework.ResolvedProfile, allowUnconfirmed bool) (runInputs, sourcereview.Result, error) {
	review, err := sourcereview.Review(inputs.model, standard, allowUnconfirmed)
	if err != nil {
		return runInputs{}, sourcereview.Result{}, fmt.Errorf("sources: %w", err)
	}

	inputs.model = review.Model

	return inputs, review, nil
}

// withSources records what reviewing the sources found beside the receiver
// set's own counts, so provenance and the run summary carry it as they carry
// the facade counts. The metadata map is the set's own, so the returned set
// and s share it.
func (s receiverSet) withSources(review sourcereview.Result, allowUnconfirmed bool) receiverSet {
	s.sourceProposed = review.Proposed
	s.sourceRejected = len(review.Excluded)

	for key, count := range map[string]int{
		sourceProposedCountKey: s.sourceProposed,
		sourceRejectedCountKey: s.sourceRejected,
	} {
		if count > 0 {
			s.metadata[key] = strconv.Itoa(count)
		}
	}

	if allowUnconfirmed && review.Proposed > 0 {
		s.metadata[allowUnconfirmedKey] = "true"
	}

	return s
}
