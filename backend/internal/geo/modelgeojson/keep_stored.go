package modelgeojson

import (
	"math"
	"reflect"
	"slices"

	"github.com/aconiq/backend/internal/geo"
)

// KeepStoredCoordinates returns incoming with the stored coordinates put back
// wherever a feature arrived where it already was: matched to a stored feature
// by id, of the same kind and geometry type, with the same coordinate tree, and
// every vertex within toleranceM of the stored one. Each coordinate-bearing
// property (propertyGeometries: a facade's from / to above all) is matched the
// same way, on its own, so a receiver moved on the map keeps the endpoints of
// the building edge it belongs to. Every other property is the incoming one's,
// so an edit to anything but a coordinate lands.
//
// It exists for a save that reached the server through another CRS. The map
// draws in EPSG:4326, and a projected project's model comes back from there
// with every vertex moved by nanometres (PLAN.md §1.6). That is harmless to a
// level and not to a facade key: part, ring and edge are canonical in the
// compute CRS, and a nanometre breaks a tie on x the other way, renumbering
// every edge of the footprint (facade-receivers.md, The facade key). Putting
// the stored bits back renumbers nothing a person did not move.
//
// A geographic project CRS measures in degrees, where a tolerance in metres
// means nothing, so there it returns incoming as it is. Neither model is
// written through.
func KeepStoredCoordinates(incoming, stored Model, toleranceM float64) Model {
	crs, err := geo.ParseCRS(stored.ProjectCRS)
	if err != nil || crs.Kind == geo.CRSKindGeographic || incoming.ProjectCRS != stored.ProjectCRS {
		return incoming
	}

	byID := make(map[string]Feature, len(stored.Features))
	duplicated := make(map[string]bool)

	for _, feature := range stored.Features {
		if _, seen := byID[feature.ID]; seen {
			duplicated[feature.ID] = true
		}

		byID[feature.ID] = feature
	}

	out := incoming
	out.Features = slices.Clone(incoming.Features)

	for i, feature := range out.Features {
		previous, ok := byID[feature.ID]
		if !ok || duplicated[feature.ID] || previous.Kind != feature.Kind {
			continue
		}

		if previous.GeometryType == feature.GeometryType && coordinatesNear(feature.Coordinates, previous.Coordinates, toleranceM) {
			out.Features[i].Coordinates = previous.Coordinates
		}

		out.Features[i].Properties = keepStoredPropertyGeometries(feature.Properties, previous.Properties, toleranceM)
	}

	return out
}

// keepStoredPropertyGeometries returns props with each coordinate-bearing
// container that matches the stored one replaced by the stored one. A match
// means equal in everything but the coordinates, so the replacement changes
// nothing but bits below the tolerance. props is copied before the first
// write.
func keepStoredPropertyGeometries(props, stored map[string]any, toleranceM float64) map[string]any {
	out := props
	copied := false

	for _, spec := range propertyGeometries {
		value, ok := props[spec.container]
		if !ok {
			continue
		}

		previous, ok := stored[spec.container]
		if !ok || !containerNear(value, previous, spec, toleranceM) {
			continue
		}

		if !copied {
			out = cloneProperties(props)
			copied = true
		}

		out[spec.container] = previous
	}

	return out
}

// containerNear reports whether two values of one propertyGeometries container
// differ in nothing but coordinates within toleranceM.
func containerNear(a, b any, spec propertyGeometry, toleranceM float64) bool {
	if spec.singleton {
		return entryNear(a, b, spec, toleranceM)
	}

	entriesA, okA := a.([]any)
	entriesB, okB := b.([]any)

	if !okA || !okB || len(entriesA) != len(entriesB) {
		return false
	}

	for i := range entriesA {
		if !entryNear(entriesA[i], entriesB[i], spec, toleranceM) {
			return false
		}
	}

	return true
}

// entryNear compares one object of a container: the members spec names as
// coordinates within toleranceM, every other member exactly. Anything that is
// not an object is compared exactly, as reprojection carries it through.
func entryNear(a, b any, spec propertyGeometry, toleranceM float64) bool {
	entryA, okA := a.(map[string]any)
	entryB, okB := b.(map[string]any)

	if !okA || !okB {
		return reflect.DeepEqual(a, b)
	}

	if len(entryA) != len(entryB) {
		return false
	}

	for member, valueA := range entryA {
		valueB, ok := entryB[member]
		if !ok {
			return false
		}

		switch {
		case slices.Contains(spec.nested, member):
			if !coordinatesNear(valueA, valueB, toleranceM) {
				return false
			}
		case member == spec.pointXY[0] || member == spec.pointXY[1]:
			// Compared as a pair below.
		default:
			if !reflect.DeepEqual(valueA, valueB) {
				return false
			}
		}
	}

	if spec.pointXY[0] == "" {
		return true
	}

	return coordinatesNear(
		[]any{entryA[spec.pointXY[0]], entryA[spec.pointXY[1]]},
		[]any{entryB[spec.pointXY[0]], entryB[spec.pointXY[1]]},
		toleranceM,
	)
}

// coordinatesNear reports whether two coordinate trees have the same shape and
// every [x, y] of one lies within toleranceM of the other's. A third ordinate
// is an elevation no CRS round trip touches, so it must be equal.
func coordinatesNear(a, b any, toleranceM float64) bool {
	listA, okA := a.([]any)
	listB, okB := b.([]any)

	if !okA || !okB {
		return reflect.DeepEqual(a, b)
	}

	if len(listA) != len(listB) {
		return false
	}

	if isCoordinatePair(listA) {
		if !isCoordinatePair(listB) {
			return false
		}

		ax, _ := listA[0].(float64)
		ay, _ := listA[1].(float64)
		bx, _ := listB[0].(float64)
		by, _ := listB[1].(float64)

		return math.Hypot(ax-bx, ay-by) <= toleranceM && reflect.DeepEqual(listA[2:], listB[2:])
	}

	for i := range listA {
		if !coordinatesNear(listA[i], listB[i], toleranceM) {
			return false
		}
	}

	return true
}
