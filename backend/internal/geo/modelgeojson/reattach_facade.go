package modelgeojson

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/aconiq/backend/internal/geo"
)

// The outcomes of re-attaching one facade receiver.
const (
	// ReattachProposed: exactly one edge matches, and the receiver can be
	// re-keyed to it.
	ReattachProposed = "proposed"
	// ReattachAmbiguous: several buildings have a matching edge. None is
	// chosen; Candidates names them.
	ReattachAmbiguous = "ambiguous"
	// ReattachNoMatch: no edge matches within the tolerance.
	ReattachNoMatch = "no-match"
	// ReattachUnchecked: there is no metric frame to match in, or a point
	// cannot be projected into or out of it.
	ReattachUnchecked = "unchecked"
)

// FacadeReattachment is what re-attaching found for one receiver whose facade
// link validation reports as unresolved or moved.
//
// Proposed, ShiftM and ReceiverDistanceM are set for a proposal only. Keys
// are in the project CRS, as the model stores them; the distances are metres
// in the frame validation measures in.
type FacadeReattachment struct {
	ReceiverID string        `json:"receiver_id"`
	Finding    string        `json:"finding"`
	Outcome    string        `json:"outcome"`
	Previous   geo.FacadeKey `json:"previous"`

	Proposed *geo.FacadeKey `json:"proposed,omitempty"`
	// ShiftM is how far the matched edge's endpoints lie from the stored
	// from / to, the larger of the two.
	ShiftM *float64 `json:"shift_m,omitempty"`
	// ReceiverDistanceM is the receiver's distance from the matched edge, so a
	// reviewer sees a point that no longer stands where its rule puts it.
	ReceiverDistanceM *float64 `json:"receiver_distance_m,omitempty"`

	Candidates []geo.FacadeKey `json:"candidates,omitempty"`
	Reason     string          `json:"reason,omitempty"`
}

// ProposeFacadeReattachments proposes, for every receiver whose facade link
// validation reports as unresolved or moved, the building edge it now
// belongs to (facade-receivers.md, Decision: what keys a building).
//
// A receiver keeps its from / to and nothing of its building's old footprint,
// so the endpoints decide. An edge matches when its start lies within
// maxShiftM of the stored from and its end within maxShiftM of the stored to.
// The comparison is directed: the neighbour across a party wall has the same
// edge walked the other way and never matches. The receiver's own building is
// searched first, so a renumbered edge stays on it; only when it has no match
// are the others searched, in model order. One match is a proposal, several
// are ambiguous and none is chosen.
//
// The matching is in the frame validation measures in: the project CRS when it
// is projected, the compute CRS `aconiq run` would choose otherwise. A model
// with validation errors is refused: its ids, keys or footprints are not
// sound enough to re-key anything against.
func ProposeFacadeReattachments(model Model, maxShiftM float64) ([]FacadeReattachment, error) {
	if !(maxShiftM > 0) || math.IsInf(maxShiftM, 0) {
		return nil, fmt.Errorf("maximum shift %v m must be a finite number > 0", maxShiftM)
	}

	report := Validate(model)
	if report.ErrorCount() > 0 {
		return nil, fmt.Errorf("the model has %d validation error(s); re-attaching needs a model that validates", report.ErrorCount())
	}

	findings := make(map[string]string)

	for _, issue := range report.Warnings {
		if issue.Code == CodeReceiverFacadeUnresolved || issue.Code == CodeReceiverFacadeMoved {
			findings[issue.FeatureID] = issue.Code
		}
	}

	if len(findings) == 0 {
		return nil, nil
	}

	index := newReattachIndex(model)

	var out []FacadeReattachment

	for _, feature := range model.Features {
		id := strings.TrimSpace(feature.ID)

		code, flagged := findings[id]
		if !flagged || featureKind(feature) != FeatureKindReceiver {
			continue
		}

		key, problem := parseFacadeKey(feature.Properties[PropertyFacade])
		if problem != "" {
			continue
		}

		out = append(out, index.reattach(feature, FacadeReattachment{ReceiverID: id, Finding: code, Previous: key}, maxShiftM))
	}

	return out, nil
}

// ApplyFacadeReattachments re-keys every proposed receiver and marks it
// proposed, so a person confirms it before a normative run assesses it. Its
// id, position, height, storey and placement rule stay as they are: a run
// never recomputes a position from the key. The model passed in is not
// modified. It returns the new model and the number of receivers re-keyed.
func ApplyFacadeReattachments(model Model, reattachments []FacadeReattachment) (Model, int) {
	proposed := make(map[string]geo.FacadeKey)

	for _, r := range reattachments {
		if r.Outcome == ReattachProposed && r.Proposed != nil {
			proposed[r.ReceiverID] = *r.Proposed
		}
	}

	out := model
	out.Features = slices.Clone(model.Features)
	applied := 0

	for i, feature := range out.Features {
		key, ok := proposed[strings.TrimSpace(feature.ID)]
		if !ok || featureKind(feature) != FeatureKindReceiver {
			continue
		}

		props := maps.Clone(feature.Properties)
		props[PropertyFacade] = facadeKeyProperty(key)
		props[PropertyReviewStatus] = geo.ReviewStatusProposed
		feature.Properties = props
		out.Features[i] = feature
		applied++
	}

	return out, applied
}

// facadeKeyProperty spells a key the way a model read from disk holds it, the
// shape parseFacadeKey reads and Reproject moves.
func facadeKeyProperty(key geo.FacadeKey) map[string]any {
	return map[string]any{
		"building_ref":   key.BuildingRef,
		"part":           float64(key.Part),
		"ring":           float64(key.Ring),
		"edge":           float64(key.Edge),
		facadeFromMember: []any{key.From.X, key.From.Y},
		facadeToMember:   []any{key.To.X, key.To.Y},
	}
}

// reattachIndex holds every building that can carry facade receivers, with
// its canonical edges in the metric frame, in model order.
type reattachIndex struct {
	frame     metricFrame
	buildings []indexedBuilding
	byID      map[string]int
}

type indexedBuilding struct {
	id      string
	feature Feature
	edges   []geo.CanonicalFacadeEdge
}

type edgeMatch struct {
	building int
	edge     geo.CanonicalFacadeEdge
	shiftM   float64
}

// newReattachIndex canonicalises each building once. A building that has no
// stable id, repeats an id, or whose footprint cannot be projected or
// canonicalised is no candidate: validation reports what is wrong with it.
func newReattachIndex(model Model) reattachIndex {
	index := reattachIndex{frame: newMetricFrame(model), byID: make(map[string]int)}
	if index.frame.failure != "" {
		return index
	}

	for _, feature := range model.Features {
		id := strings.TrimSpace(feature.ID)
		if featureKind(feature) != FeatureKindBuilding || id == "" || cityGMLFallbackID.MatchString(id) {
			continue
		}

		if _, seen := index.byID[id]; seen {
			continue
		}

		measured := feature

		if index.frame.pipeline != nil {
			projected, err := index.frame.building(id, feature)
			if err != nil {
				continue
			}

			measured = projected
		}

		polygons, err := featurePolygons(measured)
		if err != nil {
			continue
		}

		edges, err := geo.FacadeEdges(polygons)
		if err != nil {
			continue
		}

		index.byID[id] = len(index.buildings)
		index.buildings = append(index.buildings, indexedBuilding{id: id, feature: feature, edges: edges})
	}

	return index
}

func (x reattachIndex) reattach(receiver Feature, r FacadeReattachment, maxShiftM float64) FacadeReattachment {
	if x.frame.failure != "" {
		return unchecked(r, x.frame.failure)
	}

	from, fromErr := x.toMetric(r.Previous.From)
	to, toErr := x.toMetric(r.Previous.To)

	if err := errors.Join(fromErr, toErr); err != nil {
		return unchecked(r, fmt.Sprintf("the stored from / to cannot be projected into %s (%v)", x.frame.crsID, err))
	}

	matches := x.matches(r.Previous.BuildingRef, from, to, maxShiftM)

	switch len(matches) {
	case 0:
		r.Outcome = ReattachNoMatch
		r.Reason = fmt.Sprintf("no building edge runs from within %g m of the stored from to within %g m of the stored to%s",
			maxShiftM, maxShiftM, x.measuredIn())

		return r
	case 1:
		return x.propose(receiver, r, matches[0])
	default:
		return x.ambiguous(r, matches)
	}
}

func unchecked(r FacadeReattachment, reason string) FacadeReattachment {
	r.Outcome = ReattachUnchecked
	r.Reason = "cannot be checked: " + reason

	return r
}

// matches searches the building the key names first, and every other one
// only when it has no match.
func (x reattachIndex) matches(ref string, from, to geo.Point2D, maxShiftM float64) []edgeMatch {
	own, hasOwn := x.byID[ref]
	if hasOwn {
		if matches := x.matchesOn(own, from, to, maxShiftM); len(matches) > 0 {
			return matches
		}
	}

	var matches []edgeMatch

	for i := range x.buildings {
		if !hasOwn || i != own {
			matches = append(matches, x.matchesOn(i, from, to, maxShiftM)...)
		}
	}

	return matches
}

func (x reattachIndex) propose(receiver Feature, r FacadeReattachment, match edgeMatch) FacadeReattachment {
	key, err := x.keyOf(match)
	if err != nil {
		return unchecked(r, err.Error())
	}

	// The key written back must be one validation accepts, after the round
	// trip into the project CRS, or applying it would only move the finding.
	if reason, moved := x.frame.facadeMoved(x.buildings[match.building].feature, key); moved {
		return unchecked(r, "the re-attached key would not validate: "+reason)
	}

	shift := match.shiftM
	r.Outcome = ReattachProposed
	r.Proposed = &key
	r.ShiftM = &shift

	if position, ok := asPosition(receiver.Coordinates); ok {
		if p, err := x.toMetric(position); err == nil {
			distance := geo.DistancePointToSegment(p, match.edge.From, match.edge.To)
			r.ReceiverDistanceM = &distance
		}
	}

	return r
}

func (x reattachIndex) ambiguous(r FacadeReattachment, matches []edgeMatch) FacadeReattachment {
	r.Candidates = make([]geo.FacadeKey, 0, len(matches))

	for _, m := range matches {
		key, err := x.keyOf(m)
		if err != nil {
			r.Candidates = nil

			return unchecked(r, err.Error())
		}

		r.Candidates = append(r.Candidates, key)
	}

	r.Outcome = ReattachAmbiguous
	r.Reason = fmt.Sprintf("%d building edges match; name the right one by editing the receiver's facade", len(matches))

	return r
}

// matchesOn lists the edges of one building that run from near from to near
// to, in canonical order.
func (x reattachIndex) matchesOn(building int, from, to geo.Point2D, maxShiftM float64) []edgeMatch {
	var matches []edgeMatch

	for _, edge := range x.buildings[building].edges {
		fromOff, toOff := geo.Distance(edge.From, from), geo.Distance(edge.To, to)
		if fromOff <= maxShiftM && toOff <= maxShiftM {
			matches = append(matches, edgeMatch{building: building, edge: edge, shiftM: max(fromOff, toOff)})
		}
	}

	return matches
}

// keyOf is the key a matched edge is written into the model with: its
// indices, and its endpoints in the project CRS.
func (x reattachIndex) keyOf(m edgeMatch) (geo.FacadeKey, error) {
	from, fromErr := x.toProject(m.edge.From)
	to, toErr := x.toProject(m.edge.To)

	if err := errors.Join(fromErr, toErr); err != nil {
		return geo.FacadeKey{}, fmt.Errorf("the matched edge cannot be projected back from %s (%w)", x.frame.crsID, err)
	}

	return geo.FacadeKey{
		BuildingRef: x.buildings[m.building].id,
		Part:        m.edge.Part, Ring: m.edge.Ring, Edge: m.edge.Edge,
		From: from, To: to,
	}, nil
}

func (x reattachIndex) toMetric(p geo.Point2D) (geo.Point2D, error) {
	if x.frame.pipeline == nil {
		return p, nil
	}

	projected, err := x.frame.pipeline.ApplyPoint(p)
	if err != nil {
		return geo.Point2D{}, fmt.Errorf("project into %s: %w", x.frame.crsID, err)
	}

	return projected, nil
}

func (x reattachIndex) toProject(p geo.Point2D) (geo.Point2D, error) {
	if x.frame.pipeline == nil {
		return p, nil
	}

	if x.frame.inverse == nil {
		return geo.Point2D{}, x.frame.inverseErr
	}

	projected, err := x.frame.inverse.ApplyPoint(p)
	if err != nil {
		return geo.Point2D{}, fmt.Errorf("project out of %s: %w", x.frame.crsID, err)
	}

	return projected, nil
}

func (x reattachIndex) measuredIn() string {
	if x.frame.crsID == "" {
		return ""
	}

	return " (measured in " + x.frame.crsID + ")"
}
