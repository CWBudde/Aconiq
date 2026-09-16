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

// Reproject returns the model with every coordinate transformed from its own
// ProjectCRS into targetCRS, and ProjectCRS restated as targetCRS.
//
// Only x and y move. A third ordinate is an absolute elevation in metres and
// is carried through untouched, which is what transformPoint already does.
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

		reprojected := feature
		reprojected.Coordinates = coords
		out.Features = append(out.Features, reprojected)
	}

	return out, nil
}
