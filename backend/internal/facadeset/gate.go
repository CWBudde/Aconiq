package facadeset

import (
	"fmt"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/standards/framework"
)

// Gate refuses a normative run over facade points it must not assess
// (facade-receivers.md, Decision: what the run does with `proposed`).
//
// A broken facade link — a building that no longer resolves, or an edge that
// moved — and a point placed by another standard's rule refuse the run
// outright: the opt-in does not lift them, because re-attaching a point is a
// reviewed step and the opt-in is not one. Proposed points and assumed slab
// heights refuse it unless the run opts in. A non-normative standard assesses
// nothing, so it runs over all of them.
func Gate(listed Listed, standard framework.ResolvedProfile, model modelgeojson.Model, allowUnconfirmed bool) error {
	if standard.EvidenceTier != framework.EvidenceTierNormative || len(listed.Facade) == 0 {
		return nil
	}

	blocked := brokenLinks(listed, standard, model)
	if len(blocked) > 0 {
		return domainerrors.New(domainerrors.KindUserInput, op, fmt.Sprintf(
			"standard %q is normative and %d facade receiver finding(s) keep it from assessing them: %s; "+
				"re-attach or remove those receivers, --allow-unconfirmed does not apply to these",
			standard.StandardID, len(blocked), summarize(blocked, 5),
		), nil)
	}

	proposed, assumed := listed.CountUnconfirmed()
	if allowUnconfirmed || proposed+assumed == 0 {
		return nil
	}

	return domainerrors.New(domainerrors.KindUserInput, op, fmt.Sprintf(
		"standard %q is normative and its receivers include %d proposed facade point(s) and %d with an assumed slab height: "+
			"nobody has confirmed where these Immissionsorte are, so their levels are not an assessment; "+
			"confirm them in the model (review_status: confirmed), or pass --allow-unconfirmed to run anyway and have the counts recorded with the result",
		standard.StandardID, proposed, assumed,
	), nil)
}

// brokenLinks lists why a normative standard must not compute at some of the
// listed receivers: validation found their facade link broken, or another
// standard's rule placed them. Validation runs again here, on the model in the
// compute CRS, so its 1 mm tolerance is metres even when the project CRS is
// geographic.
func brokenLinks(listed Listed, standard framework.ResolvedProfile, model modelgeojson.Model) []string {
	findings := make(map[string][]string)

	for _, issue := range modelgeojson.Validate(model).Warnings {
		if issue.Code == modelgeojson.CodeReceiverFacadeUnresolved || issue.Code == modelgeojson.CodeReceiverFacadeMoved {
			findings[issue.FeatureID] = append(findings[issue.FeatureID], issue.Code+": "+issue.Message)
		}
	}

	var blocked []string

	for _, receiver := range listed.Receivers {
		attrs, ok := listed.Facade[receiver.ID]
		if !ok {
			continue
		}

		blocked = append(blocked, findings[receiver.ID]...)

		if attrs.PlacementRule == "" {
			continue
		}

		if owner := RuleStandard(attrs.PlacementRule); owner != standard.StandardID {
			if owner == "" {
				owner = "no standard"
			}

			blocked = append(blocked, fmt.Sprintf(
				"receiver %q: placement_rule %q places an Immissionsort of %s, not of %s",
				receiver.ID, attrs.PlacementRule, owner, standard.StandardID,
			))
		}
	}

	return blocked
}

// summarize joins the first limit findings, and says how many more there are.
func summarize(findings []string, limit int) string {
	if len(findings) <= limit {
		return strings.Join(findings, "; ")
	}

	return strings.Join(findings[:limit], "; ") + fmt.Sprintf(" (+%d more)", len(findings)-limit)
}
