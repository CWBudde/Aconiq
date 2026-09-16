package modelgeojson

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aconiq/backend/internal/geo"
)

// walkCoordinates visits every [x, y] pair in a GeoJSON coordinate tree. It
// mirrors transformCoordinates' recursion rather than sharing it, because that
// one rebuilds the tree and this one only reads it.
func walkCoordinates(coords any, visit func(x, y float64)) {
	values, ok := coords.([]any)
	if !ok || len(values) == 0 {
		return
	}

	if isCoordinatePair(values) {
		x, _ := values[0].(float64)
		y, _ := values[1].(float64)

		visit(x, y)

		return
	}

	for _, elem := range values {
		walkCoordinates(elem, visit)
	}
}

// Bounds returns the extent of every coordinate in the model, in the model's
// own CRS. The second result reports whether the model had any coordinate at
// all; an empty model has no extent rather than a zero one.
func (m Model) Bounds() (geo.BBox, bool) {
	var (
		bbox  geo.BBox
		found bool
	)

	for _, feature := range m.Features {
		walkCoordinates(feature.Coordinates, func(x, y float64) {
			if !found {
				bbox = geo.BBox{MinX: x, MinY: y, MaxX: x, MaxY: y}
				found = true

				return
			}

			bbox.MinX = min(bbox.MinX, x)
			bbox.MinY = min(bbox.MinY, y)
			bbox.MaxX = max(bbox.MaxX, x)
			bbox.MaxY = max(bbox.MaxY, y)
		})
	}

	if !found || !bbox.IsFinite() {
		return geo.BBox{}, false
	}

	return bbox, true
}

// propertyGeometry names a property that carries coordinates in the project
// CRS rather than in Feature.Coordinates, and says which shape they take.
//
// These exist because two standards vocabularies attach geometry to a feature
// through its properties instead of its geometry: RLS-19 directional sources
// carry their own centerline, and Schall 03 track features carry a point each.
// Both are consumed as geometry by run_extract_rls19.go and
// run_extract_schall03_normative.go, so a reprojection that moved only
// Feature.Coordinates would leave them in degrees while everything around them
// moved into metres — placing directional sources millions of metres from
// their own receivers, and pushing track features past the 25 m proximity
// check they are validated against.
//
// **Adding a coordinate-bearing property to the v1 schema means adding it
// here.** Nothing detects the omission automatically; it shows up as geometry
// in the wrong hemisphere.
type propertyGeometry struct {
	// container is the top-level property holding an array of objects.
	container string
	// nested names the members of each object that hold a coordinate tree
	// (a GeoJSON-shaped nesting of [x, y] pairs).
	nested []string
	// pointXY names a pair of scalar members holding one coordinate.
	pointXY [2]string
}

var propertyGeometries = []propertyGeometry{
	{container: "rls19_directional_sources", nested: []string{"centerline", "coordinates"}},
	{container: "schall03_track_features", pointXY: [2]string{"x", "y"}},
}

// reprojectProperties returns properties whose embedded coordinates have been
// transformed. It copies every container it touches rather than writing
// through, so the model handed in is left as it was.
func reprojectProperties(props map[string]any, pipeline *geo.TransformPipeline) (map[string]any, error) {
	if len(props) == 0 {
		return props, nil
	}

	out := props
	copied := false

	for _, spec := range propertyGeometries {
		entries, ok := out[spec.container].([]any)
		if !ok {
			continue
		}

		converted, err := reprojectPropertyEntries(entries, spec, pipeline)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", spec.container, err)
		}

		// Copy on first write, so a model with no embedded geometry shares its
		// properties map and one that has some does not write through it.
		if !copied {
			out = cloneProperties(props)
			copied = true
		}

		out[spec.container] = converted
	}

	return out, nil
}

func reprojectPropertyEntries(entries []any, spec propertyGeometry, pipeline *geo.TransformPipeline) ([]any, error) {
	converted := make([]any, len(entries))

	for i, rawEntry := range entries {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			converted[i] = rawEntry

			continue
		}

		next := cloneProperties(entry)

		for _, member := range spec.nested {
			value, exists := next[member]
			if !exists {
				continue
			}

			moved, err := transformCoordinates(value, pipeline)
			if err != nil {
				return nil, fmt.Errorf("[%d].%s: %w", i, member, err)
			}

			next[member] = moved
		}

		if spec.pointXY[0] != "" {
			err := reprojectPointXY(next, spec.pointXY, pipeline)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
		}

		converted[i] = next
	}

	return converted, nil
}

// reprojectPointXY moves a pair of scalar members. A pair where either member
// is missing or is not a number is left alone: extraction reports that as the
// validation error it is, and inventing a coordinate here would hide it.
func reprojectPointXY(entry map[string]any, members [2]string, pipeline *geo.TransformPipeline) error {
	x, xOK := entry[members[0]].(float64)
	y, yOK := entry[members[1]].(float64)

	if !xOK || !yOK {
		return nil
	}

	moved, err := pipeline.ApplyPoint(geo.Point2D{X: x, Y: y})
	if err != nil {
		return fmt.Errorf("point (%.6f, %.6f): %w", x, y, err)
	}

	entry[members[0]] = moved.X
	entry[members[1]] = moved.Y

	return nil
}

// Reproject returns the model with every coordinate transformed from its own
// ProjectCRS into targetCRS, and ProjectCRS restated as targetCRS.
//
// Only x and y move. A third ordinate is an absolute elevation in metres and
// is carried through untouched, which is what transformPoint already does.
//
// Coordinates embedded in properties move too — see propertyGeometries for
// which ones and why.
//
// ImportCRS and TransformApplied are left exactly as they were: they record
// where the model's coordinates came from at import time, and reprojecting a
// loaded model does not change that history.
func Reproject(model Model, targetCRS string) (Model, error) {
	target := strings.TrimSpace(targetCRS)
	source := strings.TrimSpace(model.ProjectCRS)

	if target == "" {
		return Model{}, errors.New("target CRS is required")
	}

	if source == "" {
		return Model{}, errors.New("model has no project CRS to reproject from")
	}

	if strings.EqualFold(source, target) {
		return model, nil
	}

	from, err := geo.ParseCRS(source)
	if err != nil {
		return Model{}, fmt.Errorf("parse project CRS %q: %w", source, err)
	}

	to, err := geo.ParseCRS(target)
	if err != nil {
		return Model{}, fmt.Errorf("parse target CRS %q: %w", target, err)
	}

	pipeline, err := geo.BuildTransformPipeline(to, from)
	if err != nil {
		return Model{}, fmt.Errorf("build CRS transform %s -> %s: %w", source, target, err)
	}

	out := model
	out.ProjectCRS = to.ID
	out.Features = make([]Feature, 0, len(model.Features))

	for _, feature := range model.Features {
		coords, err := transformCoordinates(feature.Coordinates, &pipeline)
		if err != nil {
			return Model{}, fmt.Errorf("feature %q: CRS transform: %w", feature.ID, err)
		}

		properties, err := reprojectProperties(feature.Properties, &pipeline)
		if err != nil {
			return Model{}, fmt.Errorf("feature %q: CRS transform: %w", feature.ID, err)
		}

		reprojected := feature
		reprojected.Coordinates = coords
		reprojected.Properties = properties
		out.Features = append(out.Features, reprojected)
	}

	return out, nil
}
