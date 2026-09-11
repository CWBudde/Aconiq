package cli

import (
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
)

type rls19LineGeometry struct {
	Centerline           []geo.Point2D
	CenterlineElevations []float64
}

type rls19DirectionalSourceSpec struct {
	IDHint    string
	Geometry  rls19LineGeometry
	Overrides map[string]any
}

func extractDummySources(model modelgeojson.Model, emissionDB float64, supportedSourceTypes []string) ([]freefield.Source, error) {
	return extractSources(model, supportedSourceTypes, sourceExtraction[freefield.Source, geo.Point2D]{
		scope:        "cli.extractDummySources",
		idPrefix:     "source-%03d",
		emptyMessage: "model does not contain any supported source features",
		parts: func(feature modelgeojson.Feature) ([]geo.Point2D, error) {
			return sourcePointsFromFeature(feature, freefield.StandardID)
		},
		build: func(_ modelgeojson.Feature, sourceID string, point geo.Point2D) (freefield.Source, error) {
			return freefield.Source{
				ID:         sourceID,
				Point:      point,
				EmissionDB: emissionDB,
			}, nil
		},
	})
}
