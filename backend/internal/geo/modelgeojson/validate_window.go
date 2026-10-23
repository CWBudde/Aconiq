package modelgeojson

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/aconiq/backend/internal/geo"
)

// Validation codes for window features (P18 C1).
//
// CodeWindowInvalid is an error: a window whose facade key, frame or review
// properties do not have their documented shape cannot be placed at.
//
// The other four are warnings, for the reason a receiver's are
// (validate_facade.go): they describe a window whose building changed under
// it, and the import that changed it has to succeed. Facade receiver mode
// refuses to place at such a window; re-attaching it is a reviewed step.
// CodeWindowAboveBuilding is a window whose head rises above its building's
// height_m: no storey holds it, and a point at it would stand above the roof
// the model gives the building.
const (
	CodeWindowInvalid          = "window.invalid"
	CodeWindowFacadeUnresolved = "window.facade.unresolved"
	CodeWindowFacadeMoved      = "window.facade.moved"
	CodeWindowOutsideFacade    = "window.frame.outside"
	CodeWindowAboveBuilding    = "window.frame.above_building"
)

// Window is a window feature as facade receiver mode reads it. ReviewStatus
// is proposed when the feature carries none.
type Window struct {
	ID           string
	Facade       geo.FacadeKey
	Frame        geo.FacadeWindow
	Sensitive    bool
	ReviewStatus string
}

// WindowOf reads a window feature's properties. Validation has already
// refused a malformed one, so an error here is a model that reached the run
// unvalidated.
func WindowOf(feature Feature) (Window, error) {
	key, problem := parseFacadeKey(feature.Properties[PropertyFacade])
	if problem != "" {
		return Window{}, fmt.Errorf("window %q: facade %s", feature.ID, problem)
	}

	frame, problem := parseWindowFrame(feature.Properties[PropertyWindow])
	if problem != "" {
		return Window{}, fmt.Errorf("window %q: window %s", feature.ID, problem)
	}

	window := Window{
		ID:           strings.TrimSpace(feature.ID),
		Facade:       key,
		Frame:        frame,
		ReviewStatus: geo.ReviewStatusProposed,
	}

	if v, ok := presentProperty(feature.Properties, PropertySensitive); ok {
		sensitive, isBool := v.(bool)
		if !isBool {
			return Window{}, fmt.Errorf("window %q: sensitive %#v is not a boolean", feature.ID, v)
		}

		window.Sensitive = sensitive
	}

	if v, ok := presentProperty(feature.Properties, PropertyReviewStatus); ok {
		status, isString := v.(string)
		if !isString || !slices.Contains(reviewStatuses, status) {
			return Window{}, fmt.Errorf("window %q: review_status %#v is not one of %s", feature.ID, v, strings.Join(reviewStatuses, "|"))
		}

		window.ReviewStatus = status
	}

	return window, nil
}

// validateWindowProperties checks a window's properties, one issue per
// malformed one, and returns its key and frame when both are well formed.
func validateWindowProperties(feature Feature, featureID string, report *ValidationReport) (geo.FacadeKey, geo.FacadeWindow, bool) {
	props := feature.Properties
	invalid := func(problem string) {
		addError(report, CodeWindowInvalid, featureID, fmt.Sprintf("window %q: %s", featureID, problem))
	}

	// A point at a window is named <set>/<building>/p…r…e…/w/<window>.
	if strings.Contains(featureID, "/") {
		invalid("id must not contain '/', which separates the parts of the receiver ids placed at it")
	}

	if v, ok := presentProperty(props, PropertySensitive); ok {
		if _, isBool := v.(bool); !isBool {
			invalid(fmt.Sprintf("%s %#v must be true or false", PropertySensitive, v))
		}
	}

	if v, ok := presentProperty(props, PropertyReviewStatus); ok {
		if s, isString := v.(string); !isString || !slices.Contains(reviewStatuses, s) {
			invalid(fmt.Sprintf("%s %#v must be one of %s", PropertyReviewStatus, v, strings.Join(reviewStatuses, "|")))
		}
	}

	if v, ok := presentProperty(props, PropertyEvidence); ok {
		if problem := evidenceProblem(v); problem != "" {
			invalid(PropertyEvidence + " " + problem)
		}
	}

	frame, frameProblem := parseWindowFrame(props[PropertyWindow])
	if frameProblem != "" {
		invalid(PropertyWindow + " " + frameProblem)
	}

	key, keyProblem := geo.FacadeKey{}, "is required: a window belongs to one facade"
	if v, ok := presentProperty(props, PropertyFacade); ok {
		key, keyProblem = parseFacadeKey(v)
	}

	if keyProblem != "" {
		invalid(PropertyFacade + " " + keyProblem)
	}

	return key, frame, frameProblem == "" && keyProblem == ""
}

// parseWindowFrame reads the {u_m, sill_z_m, width_m, height_m} object, and
// names the first thing wrong with it.
func parseWindowFrame(value any) (geo.FacadeWindow, string) {
	if value == nil {
		return geo.FacadeWindow{}, "is required: {u_m, sill_z_m, width_m, height_m} in the facade frame"
	}

	obj, ok := value.(map[string]any)
	if !ok {
		return geo.FacadeWindow{}, fmt.Sprintf("must be an object {u_m, sill_z_m, width_m, height_m}, got %T", value)
	}

	var frame geo.FacadeWindow

	for _, member := range []struct {
		name string
		dst  *float64
	}{{"u_m", &frame.UM}, {"sill_z_m", &frame.SillZM}, {"width_m", &frame.WidthM}, {"height_m", &frame.HeightM}} {
		v, isNumber := asFiniteFloat(obj[member.name])
		if !isNumber {
			return geo.FacadeWindow{}, fmt.Sprintf("%s %#v must be a finite number", member.name, obj[member.name])
		}

		*member.dst = v
	}

	err := frame.Validate()
	if err != nil {
		return geo.FacadeWindow{}, err.Error()
	}

	return frame, ""
}

// evidenceProblem checks that evidence is a list of objects. Their fields are
// the evidence sources' (facade-receivers.md, Receiver properties), which
// nothing reads yet, so they are not checked one by one.
func evidenceProblem(value any) string {
	entries, ok := value.([]any)
	if !ok {
		return fmt.Sprintf("must be an array of objects, got %T", value)
	}

	for i, entry := range entries {
		if _, isObject := entry.(map[string]any); !isObject {
			return fmt.Sprintf("[%d] must be an object, got %T", i, entry)
		}
	}

	return ""
}

// windowFrameProperty spells a frame the way a model read from disk holds it.
func windowFrameProperty(frame geo.FacadeWindow) map[string]any {
	return map[string]any{
		"u_m":      frame.UM,
		"sill_z_m": frame.SillZM,
		"width_m":  frame.WidthM,
		"height_m": frame.HeightM,
	}
}

// WindowSpec is a window given in its facade's frame, as a window list
// states it: the building, the canonical part, ring and edge, and the
// rectangle on it. ReviewStatus is proposed when empty.
type WindowSpec struct {
	ID           string
	BuildingRef  string
	Part         int
	Ring         int
	Edge         int
	Frame        geo.FacadeWindow
	Sensitive    bool
	ReviewStatus string
	Evidence     []any
}

// WindowPlacer turns window specs into model features against one model's
// buildings, each canonicalised once.
type WindowPlacer struct {
	index reattachIndex
}

// NewWindowPlacer indexes the model's buildings in the frame validation
// measures in: the project CRS when it is projected, the compute CRS `aconiq
// run` would choose otherwise. A geographic model with no such CRS has no
// frame to place a window in.
func NewWindowPlacer(model Model) (WindowPlacer, error) {
	index := newReattachIndex(model)
	if index.frame.failure != "" {
		return WindowPlacer{}, errors.New(index.frame.failure)
	}

	return WindowPlacer{index: index}, nil
}

// Feature places one window: its facade key from the canonical edge the spec
// names, with from / to in the project CRS, and its Point at the window
// centre on the facade plane. It refuses a window its edge has no room for.
func (p WindowPlacer) Feature(spec WindowSpec) (Feature, error) {
	id := strings.TrimSpace(spec.ID)
	if id == "" || strings.Contains(id, "/") {
		return Feature{}, fmt.Errorf("window id %q must be non-empty and must not contain '/'", spec.ID)
	}

	status := spec.ReviewStatus
	if status == "" {
		status = geo.ReviewStatusProposed
	}

	if !slices.Contains(reviewStatuses, status) {
		return Feature{}, fmt.Errorf("window %q: review_status %q is not one of %s", id, status, strings.Join(reviewStatuses, "|"))
	}

	err := spec.Frame.Validate()
	if err != nil {
		return Feature{}, fmt.Errorf("window %q: %w", id, err)
	}

	match, err := p.edge(spec)
	if err != nil {
		return Feature{}, fmt.Errorf("window %q: %w", id, err)
	}

	edge := match.edge

	if length := geo.Distance(edge.From, edge.To); !spec.Frame.Fits(length) {
		return Feature{}, fmt.Errorf("window %q: u_m %g + width_m %g runs past the end of its %.3f m facade%s",
			id, spec.Frame.UM, spec.Frame.WidthM, length, p.index.measuredIn())
	}

	key, err := p.index.keyOf(match)
	if err != nil {
		return Feature{}, fmt.Errorf("window %q: %w", id, err)
	}

	centre, err := p.index.windowCentre(edge.From, edge.To, spec.Frame)
	if err != nil {
		return Feature{}, fmt.Errorf("window %q: %w", id, err)
	}

	props := map[string]any{
		PropertyFacade:       facadeKeyProperty(key),
		PropertyWindow:       windowFrameProperty(spec.Frame),
		PropertySensitive:    spec.Sensitive,
		PropertyReviewStatus: status,
	}

	if len(spec.Evidence) > 0 {
		props[PropertyEvidence] = spec.Evidence
	}

	return Feature{
		ID:           id,
		Kind:         FeatureKindWindow,
		GeometryType: GeometryTypePoint,
		Coordinates:  []any{centre.X, centre.Y},
		Properties:   props,
	}, nil
}

// PlacedEdge is one canonical edge of a building, as a window on it is keyed:
// the key, with from / to in the project CRS, and the edge's length in metres
// and the bearing it faces (geo.FacadeFacingDeg) in the frame the placer
// measures in.
type PlacedEdge struct {
	Key       geo.FacadeKey
	LengthM   float64
	FacingDeg float64
}

// Edges lists a building's canonical edges in canonical order: the facades a
// window can be placed on, and the lengths a frame's u_m and width_m are
// measured along (P18 C2).
func (p WindowPlacer) Edges(buildingRef string) ([]PlacedEdge, error) {
	building, ok := p.index.byID[buildingRef]
	if !ok {
		return nil, notIndexed(buildingRef)
	}

	edges := p.index.buildings[building].edges
	placed := make([]PlacedEdge, 0, len(edges))

	for _, e := range edges {
		key, err := p.index.keyOf(edgeMatch{building: building, edge: e})
		if err != nil {
			return nil, fmt.Errorf("building %q: %w", buildingRef, err)
		}

		placed = append(placed, PlacedEdge{Key: key, LengthM: geo.Distance(e.From, e.To), FacingDeg: geo.FacadeFacingDeg(e.From, e.To)})
	}

	return placed, nil
}

func notIndexed(buildingRef string) error {
	return fmt.Errorf("building %q is not in the model, or has no stable id to key a facade by", buildingRef)
}

// edge finds the canonical edge a spec names, in the metric frame.
func (p WindowPlacer) edge(spec WindowSpec) (edgeMatch, error) {
	building, ok := p.index.byID[spec.BuildingRef]
	if !ok {
		return edgeMatch{}, notIndexed(spec.BuildingRef)
	}

	for _, e := range p.index.buildings[building].edges {
		if e.Part == spec.Part && e.Ring == spec.Ring && e.Edge == spec.Edge {
			return edgeMatch{building: building, edge: e}, nil
		}
	}

	return edgeMatch{}, fmt.Errorf("building %q has no part %d, ring %d, edge %d", spec.BuildingRef, spec.Part, spec.Ring, spec.Edge)
}
