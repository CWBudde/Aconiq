package modelgeojson

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/aconiq/backend/internal/geo"
)

// Validation codes for the facade receiver properties
// (docs/research/facade-receivers.md, Data model).
//
// CodeReceiverFacadeInvalid is an error: a property that is present but does
// not have its documented shape is a model nothing can read honestly.
//
// CodeReceiverFacadeUnresolved and CodeReceiverFacadeMoved are warnings, and
// deliberately not errors. Both describe a receiver whose building changed
// under it, which is what re-importing a newer building source does; that
// import has to succeed, or the model could not be repaired at all. It is the
// run that refuses them, and only for a normative standard. Re-attaching such
// a receiver is an explicit, reviewed step, never a side effect of validation.
const (
	CodeReceiverFacadeInvalid    = "receiver.facade.invalid"
	CodeReceiverFacadeUnresolved = "receiver.facade.unresolved"
	CodeReceiverFacadeMoved      = "receiver.facade.moved"
)

// FacadeMoveToleranceM is how far, in metres, a facade key's stored from / to
// may lie from the endpoints of the canonical edge it names before the
// receiver counts as moved. The metres are those of the project CRS when it
// is projected, and of the compute CRS `aconiq run` would project a
// geographic project into otherwise (see metricFrame), so the check never
// compares in degrees. 1 mm is far above what a CRS round trip does to a
// coordinate (~1e-9 m) and far below any real edit of a footprint, so neither
// a reprojection nor a re-import of the same building trips it, and any change
// a person made does.
const FacadeMoveToleranceM = 0.001

// The members of a facade key that hold a position: the only ones Reproject
// moves.
const (
	facadeFromMember = "from"
	facadeToMember   = "to"
)

// maxFacadeIndex bounds the integers a facade property may carry. Nothing
// real comes near it; it keeps the float64-to-int conversion exact and in
// range on every architecture.
const maxFacadeIndex = math.MaxInt32

// cityGMLFallbackID matches the id citygmlimport mints for a building without
// a gml:id. It is an index into the file, so it does not survive a re-import,
// and such a building cannot carry facade receivers until it has a stable id.
var cityGMLFallbackID = regexp.MustCompile(`^citygml-building-[0-9]+$`)

var (
	slabHeightSources = []string{geo.SlabHeightMeasured, geo.SlabHeightLevels, geo.SlabHeightAssumed}
	reviewStatuses    = []string{geo.ReviewStatusProposed, geo.ReviewStatusConfirmed, geo.ReviewStatusRejected}
)

// facadeTracker checks facade receivers against the buildings they name. That
// needs the whole model, as calcAreaTracker does, so it collects the buildings
// and the parsed keys while Validate walks the features and resolves them
// once the walk is done.
type facadeTracker struct {
	buildings map[string]Feature
	pending   []pendingFacade
}

type pendingFacade struct {
	receiverID string
	key        geo.FacadeKey
}

// observe records a building, or checks a receiver's facade properties and
// keeps a well-formed key for resolve. A receiver carrying none of them is
// left alone.
func (t *facadeTracker) observe(feature Feature, featureID string, report *ValidationReport) {
	switch featureKind(feature) {
	case FeatureKindBuilding:
		if t.buildings == nil {
			t.buildings = make(map[string]Feature)
		}

		// A duplicate id is already feature.id.duplicate; the first one
		// stands, as it does for every other lookup by id.
		if _, seen := t.buildings[featureID]; !seen && featureID != "" {
			t.buildings[featureID] = feature
		}
	case FeatureKindReceiver:
		key, ok := validateFacadeProperties(feature, featureID, report)
		if ok {
			t.pending = append(t.pending, pendingFacade{receiverID: featureID, key: key})
		}
	}
}

// resolve reports, in feature order, every key that names no building or no
// longer names the edge it was placed on. The model is only read when there is
// a key to check, and then only for its CRS and extent.
func (t *facadeTracker) resolve(model Model, report *ValidationReport) {
	if len(t.pending) == 0 {
		return
	}

	frame := newMetricFrame(model)

	for _, p := range t.pending {
		building, ok := t.buildings[p.key.BuildingRef]
		if !ok {
			addWarning(report, CodeReceiverFacadeUnresolved, p.receiverID, fmt.Sprintf(
				"receiver %q: facade.building_ref %q names no %s feature in the model",
				p.receiverID, p.key.BuildingRef, FeatureKindBuilding,
			))

			continue
		}

		reason, moved := frame.facadeMoved(building, p.key)
		if moved {
			addWarning(report, CodeReceiverFacadeMoved, p.receiverID, fmt.Sprintf(
				"receiver %q: facade (part %d, ring %d, edge %d) of building %q %s",
				p.receiverID, p.key.Part, p.key.Ring, p.key.Edge, p.key.BuildingRef, reason,
			))
		}
	}
}

// metricFrame is where a facade key is measured against its edge.
//
// A model in a projected CRS, or in one whose kind cannot be determined, is
// measured as stored, the way `aconiq run` leaves it unprojected. A
// geographic model is measured in the compute CRS the run would project it
// into, chosen by the same rule (geo.ComputeCRSForGeographic at the centre of
// Model.Bounds), so an import or `aconiq validate` reports the receivers the
// run refuses, and no others.
//
// Only the buildings a key names and the keys themselves are projected, not
// the whole model: Validate runs on every import and save, and a city import
// carries far more buildings than facade receivers. Each coordinate goes
// through the transform Reproject applies, so the canonical footprint, and
// with it every part, ring and edge index, is the one the run builds.
type metricFrame struct {
	// pipeline projects into the compute CRS; nil measures as stored.
	pipeline *geo.TransformPipeline
	crsID    string
	// inverse projects from the compute CRS back into the project CRS, for a
	// point re-attaching writes into the model; inverseErr is why there is
	// none. Both are unset when pipeline is.
	inverse    *geo.TransformPipeline
	inverseErr error
	// failure, when set, is why a geographic model has no metric frame. Every
	// key is then reported as one that cannot be checked.
	failure string
	// buildings caches each projected building by the building_ref that
	// named it, the id facadeTracker.buildings files it under.
	buildings map[string]projectedBuilding
}

type projectedBuilding struct {
	feature Feature
	err     error
}

func newMetricFrame(model Model) metricFrame {
	source, err := geo.ParseCRS(model.ProjectCRS)
	if err != nil || source.Kind != geo.CRSKindGeographic {
		return metricFrame{}
	}

	bounds, ok := model.Bounds()
	if !ok {
		return metricFrame{failure: "the model has no finite extent to choose a metric CRS from"}
	}

	target, err := geo.ComputeCRSForGeographic((bounds.MinX+bounds.MaxX)/2, (bounds.MinY+bounds.MaxY)/2)
	if err != nil {
		return metricFrame{failure: fmt.Sprintf("%s has no metric CRS to measure in (%v)", source.ID, err)}
	}

	pipeline, err := geo.BuildTransformPipeline(target, source)
	if err != nil {
		return metricFrame{failure: fmt.Sprintf("%s cannot be projected into %s (%v)", source.ID, target.ID, err)}
	}

	frame := metricFrame{pipeline: &pipeline, crsID: target.ID, buildings: make(map[string]projectedBuilding)}

	// Only re-attaching writes a point back; validation never needs the way
	// home, so a missing one is recorded rather than failing the frame.
	inverse, err := geo.BuildTransformPipeline(source, target)
	if err != nil {
		frame.inverseErr = err
	} else {
		frame.inverse = &inverse
	}

	return frame
}

// facadeMoved is facadeMoved measured in the frame. A key that cannot be
// projected cannot be checked, which is reported the way an unreadable
// footprint is.
func (f metricFrame) facadeMoved(building Feature, key geo.FacadeKey) (string, bool) {
	if f.failure != "" {
		return "cannot be checked: " + f.failure, true
	}

	if f.pipeline == nil {
		return facadeMoved(building, key)
	}

	projected, err := f.building(key.BuildingRef, building)
	if err != nil {
		return fmt.Sprintf("cannot be checked: the building's footprint cannot be projected into %s (%v)", f.crsID, err), true
	}

	from, fromErr := f.pipeline.ApplyPoint(key.From)
	to, toErr := f.pipeline.ApplyPoint(key.To)

	if err := errors.Join(fromErr, toErr); err != nil {
		return fmt.Sprintf("cannot be checked: the stored from / to cannot be projected into %s (%v)", f.crsID, err), true
	}

	key.From, key.To = from, to

	reason, moved := facadeMoved(projected, key)
	if moved {
		reason += " (measured in " + f.crsID + ")"
	}

	return reason, moved
}

func (f metricFrame) building(ref string, building Feature) (Feature, error) {
	if cached, ok := f.buildings[ref]; ok {
		return cached.feature, cached.err
	}

	projected := building

	coords, err := transformCoordinates(building.Coordinates, f.pipeline)
	if err == nil {
		projected.Coordinates = coords
	}

	f.buildings[ref] = projectedBuilding{feature: projected, err: err}

	return projected, err
}

// facadeMoved compares a key against its building's canonical footprint, and
// says why when they no longer agree.
func facadeMoved(building Feature, key geo.FacadeKey) (string, bool) {
	polygons, err := featurePolygons(building)
	if err != nil {
		return "cannot be checked: the building's footprint does not parse (" + err.Error() + ")", true
	}

	from, to, ok := geo.FacadeEdge(polygons, key.Part, key.Ring, key.Edge)
	if !ok {
		return "is not an edge of the building's footprint", true
	}

	fromOff, toOff := geo.Distance(from, key.From), geo.Distance(to, key.To)
	if !(fromOff <= FacadeMoveToleranceM && toOff <= FacadeMoveToleranceM) {
		return fmt.Sprintf("now runs (%.3f, %.3f) → (%.3f, %.3f), %.4f m and %.4f m from the stored from / to (tolerance %g m)",
			from.X, from.Y, to.X, to.Y, fromOff, toOff, FacadeMoveToleranceM), true
	}

	return "", false
}

// featurePolygons reads a building's footprint in geo.FacadeBuilding's
// Polygons format: one entry per part, rings[0] the exterior and rings[1:]
// the holes, each ring closed as GeoJSON writes it. It parses with the same
// rules validatePolygonGeometry applies.
func featurePolygons(feature Feature) ([][][]geo.Point2D, error) {
	var raw []any

	switch feature.GeometryType {
	case GeometryTypePolygon:
		raw = []any{feature.Coordinates}
	case GeometryTypeMultiPolygon:
		parts, ok := feature.Coordinates.([]any)
		if !ok || len(parts) == 0 {
			return nil, fmt.Errorf("%s coordinates must be a non-empty array", GeometryTypeMultiPolygon)
		}

		raw = parts
	default:
		return nil, fmt.Errorf("geometry %q is not %s or %s", feature.GeometryType, GeometryTypePolygon, GeometryTypeMultiPolygon)
	}

	polygons := make([][][]geo.Point2D, len(raw))

	for i, part := range raw {
		rings, err := parsePolygon(part)
		if err != nil {
			return nil, fmt.Errorf("polygon %d: %w", i, err)
		}

		polygons[i] = make([][]geo.Point2D, len(rings))
		for r, ring := range rings {
			points := make([]geo.Point2D, len(ring))
			for k, p := range ring {
				points[k] = geo.Point2D{X: p.x, Y: p.y}
			}

			polygons[i][r] = points
		}
	}

	return polygons, nil
}

// validateFacadeProperties checks each facade property a receiver carries,
// one issue per malformed property, and returns its facade key when that is
// present and well formed. A property written as null counts as absent, the
// way an export of a nullable column writes a receiver that has none.
func validateFacadeProperties(feature Feature, featureID string, report *ValidationReport) (geo.FacadeKey, bool) {
	props := feature.Properties
	invalid := func(property, problem string) {
		addError(report, CodeReceiverFacadeInvalid, featureID, fmt.Sprintf("receiver %q: %s %s", featureID, property, problem))
	}

	if v, ok := presentProperty(props, PropertyPlacementRule); ok {
		rule, isString := v.(string)
		if !isString || !geo.IsPlacementRule(rule) {
			invalid(PropertyPlacementRule, fmt.Sprintf("%#v is not a known placement rule", v))
		}
	}

	if v, ok := presentProperty(props, PropertyStorey); ok {
		if _, isIndex := asFacadeIndex(v); !isIndex {
			invalid(PropertyStorey, fmt.Sprintf("%#v must be an integer >= 0", v))
		}
	}

	if v, ok := presentProperty(props, PropertyStationUM); ok {
		if u, isNumber := asFiniteFloat(v); !isNumber || u < 0 {
			invalid(PropertyStationUM, fmt.Sprintf("%#v must be a finite number >= 0", v))
		}
	}

	checkEnum := func(property string, allowed []string) {
		v, ok := presentProperty(props, property)
		if !ok {
			return
		}

		if s, isString := v.(string); !isString || !isOneOf(s, allowed...) {
			invalid(property, fmt.Sprintf("%#v must be one of %s", v, strings.Join(allowed, "|")))
		}
	}

	checkEnum(PropertySlabHeightSource, slabHeightSources)
	checkEnum(PropertyReviewStatus, reviewStatuses)

	v, ok := presentProperty(props, PropertyFacade)
	if !ok {
		return geo.FacadeKey{}, false
	}

	key, problem := parseFacadeKey(v)
	if problem != "" {
		invalid(PropertyFacade, problem)

		return geo.FacadeKey{}, false
	}

	return key, true
}

func presentProperty(props map[string]any, key string) (any, bool) {
	v, ok := props[key]

	return v, ok && v != nil
}

// parseFacadeKey reads the facade object in the shape geo.FacadeKey marshals
// to, and names the first thing wrong with it. from and to must be [x, y]
// arrays of float64, because those are the only ones Reproject moves; any
// other spelling would be carried through a reprojection unchanged and then
// compared against an edge in another CRS.
func parseFacadeKey(value any) (geo.FacadeKey, string) {
	obj, ok := value.(map[string]any)
	if !ok {
		return geo.FacadeKey{}, fmt.Sprintf("must be an object {building_ref, part, ring, edge, from, to}, got %T", value)
	}

	ref, ok := obj["building_ref"].(string)
	if !ok || strings.TrimSpace(ref) == "" {
		return geo.FacadeKey{}, fmt.Sprintf("building_ref %#v must be a non-empty string", obj["building_ref"])
	}

	if cityGMLFallbackID.MatchString(ref) {
		return geo.FacadeKey{}, fmt.Sprintf(
			"building_ref %q is a CityGML fallback id, an index into the file that does not survive a re-import; "+
				"the building needs a stable gml:id before it can carry facade receivers", ref,
		)
	}

	key := geo.FacadeKey{BuildingRef: ref}

	for _, member := range []struct {
		name string
		dst  *int
	}{{"part", &key.Part}, {"ring", &key.Ring}, {"edge", &key.Edge}} {
		n, isIndex := asFacadeIndex(obj[member.name])
		if !isIndex {
			return geo.FacadeKey{}, fmt.Sprintf("%s %#v must be an integer >= 0", member.name, obj[member.name])
		}

		*member.dst = n
	}

	for _, member := range []struct {
		name string
		dst  *geo.Point2D
	}{{facadeFromMember, &key.From}, {facadeToMember, &key.To}} {
		p, isPosition := asPosition(obj[member.name])
		if !isPosition {
			return geo.FacadeKey{}, fmt.Sprintf("%s %#v must be [x, y], two finite numbers", member.name, obj[member.name])
		}

		*member.dst = p
	}

	return key, ""
}

// asFacadeIndex reads a non-negative integer: a JSON number without a
// fractional part, or a Go integer from a model built in code.
func asFacadeIndex(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		if !(v >= 0 && v <= maxFacadeIndex) || v != math.Trunc(v) {
			return 0, false
		}

		return int(v), true
	case int:
		return v, v >= 0 && v <= maxFacadeIndex
	case int64:
		if v < 0 || v > maxFacadeIndex {
			return 0, false
		}

		return int(v), true
	default:
		return 0, false
	}
}

func asPosition(value any) (geo.Point2D, bool) {
	raw, ok := value.([]any)
	if !ok || len(raw) != 2 {
		return geo.Point2D{}, false
	}

	x, xOK := asFiniteFloat(raw[0])
	y, yOK := asFiniteFloat(raw[1])

	return geo.Point2D{X: x, Y: y}, xOK && yOK
}
