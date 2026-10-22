package modelgeojson

import (
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

// FacadeMoveToleranceM is how far, in metres of the project CRS, a facade
// key's stored from / to may lie from the endpoints of the canonical edge it
// names before the receiver counts as moved. 1 mm is far above what a CRS
// round trip does to a coordinate (~1e-9 m) and far below any real edit of a
// footprint, so neither a reprojection nor a re-import of the same building
// trips it, and any change a person made does.
const FacadeMoveToleranceM = 0.001

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
// longer names the edge it was placed on.
func (t *facadeTracker) resolve(report *ValidationReport) {
	for _, p := range t.pending {
		building, ok := t.buildings[p.key.BuildingRef]
		if !ok {
			addWarning(report, CodeReceiverFacadeUnresolved, p.receiverID, fmt.Sprintf(
				"receiver %q: facade.building_ref %q names no %s feature in the model",
				p.receiverID, p.key.BuildingRef, FeatureKindBuilding,
			))

			continue
		}

		reason, moved := facadeMoved(building, p.key)
		if moved {
			addWarning(report, CodeReceiverFacadeMoved, p.receiverID, fmt.Sprintf(
				"receiver %q: facade (part %d, ring %d, edge %d) of building %q %s",
				p.receiverID, p.key.Part, p.key.Ring, p.key.Edge, p.key.BuildingRef, reason,
			))
		}
	}
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
	}{{"from", &key.From}, {"to", &key.To}} {
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
