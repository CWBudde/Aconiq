package cli

import (
	"fmt"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/standards/dummy/freefield"
)

const (
	featureKindSource           = "source"
	geometryTypeLineString      = "LineString"
	geometryTypeMultiLineString = "MultiLineString"
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
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]freefield.Source, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != featureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractDummySources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		points, err := sourcePointsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractDummySources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("source-%03d", featureIndex)
		}

		for pointIndex, point := range points {
			sourceID := baseID
			if len(points) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, pointIndex+1)
			}

			sources = append(sources, freefield.Source{
				ID:         sourceID,
				Point:      point,
				EmissionDB: emissionDB,
			})
		}
	}

	if len(sources) == 0 {
		return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractDummySources", "model does not contain any supported source features", nil)
	}

	return sources, nil
}
