package cli

import (
	"errors"
	"fmt"
	"math"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
)

// rls19AcousticOverrideKeys are the feature property keys that constitute a
// per-source acoustic override for RLS-19 road. Presence of any one of these
// keys causes the source to be counted as having feature-level overrides.
var rls19AcousticOverrideKeys = []string{
	"surface_type", "road_surface_type",
	"road_speed_kph", "speed_pkw_kph", "speed_lkw1_kph", "speed_lkw2_kph", "speed_krad_kph",
	"gradient_percent", "road_gradient_percent",
	"junction_type", "road_junction_type",
	"junction_distance_m", "road_junction_distance_m",
	"building_height_m",
	"street_width_m",
	"traffic_day_pkw", "traffic_day_lkw1", "traffic_day_lkw2", "traffic_day_krad",
	"traffic_night_pkw", "traffic_night_lkw1", "traffic_night_lkw2", "traffic_night_krad",
}

// rls19FeatureHasAcousticOverrides reports whether a source feature carries any
// per-source acoustic property that would override the run-wide defaults.
func rls19FeatureHasAcousticOverrides(feature modelgeojson.Feature) bool {
	return rls19PropertiesHaveAcousticOverrides(feature.Properties)
}

func rls19PropertiesHaveAcousticOverrides(properties map[string]any) bool {
	for _, key := range rls19AcousticOverrideKeys {
		if v, ok := properties[key]; ok && v != nil {
			return true
		}
	}

	return false
}

func resolveRLS19SurfaceType(properties map[string]any, defaultSurface string) (string, error) {
	surfaceType := defaultSurface

	value, ok, err := propertyString(properties, "surface_type", "road_surface_type")
	if err != nil {
		return "", err
	}

	if ok {
		surfaceType = value
	}

	return surfaceType, nil
}

func resolveRLS19LaneCount(properties map[string]any) (int, error) {
	value, ok, err := propertyFloat(properties, "lane_count", "lanes")
	if err != nil {
		return 0, err
	}

	if !ok {
		return 0, nil
	}

	if value < 1 || math.Trunc(value) != value {
		return 0, errors.New("lane_count/lanes must be an integer >= 1")
	}

	return int(value), nil
}

func validateRLS19DirectionalSurfaceTypes(feature modelgeojson.Feature, directionalSources []rls19DirectionalSourceSpec, defaultSurface string) error {
	if len(directionalSources) <= 1 {
		return nil
	}

	resolved := make(map[string]struct{}, len(directionalSources))
	for _, directional := range directionalSources {
		surfaceType, err := resolveRLS19SurfaceType(mergedProperties(feature.Properties, directional.Overrides), defaultSurface)
		if err != nil {
			return err
		}

		resolved[surfaceType] = struct{}{}
		if len(resolved) > 1 {
			return errors.New("directional sources with different surface_type values are not supported; use one shared surface_type that already reflects the larger per-direction correction")
		}
	}

	return nil
}

// extractRLS19RoadSources extracts RLS-19 road sources from the normalized
// model, applying per-source feature properties as overrides over the run-wide
// defaults in options. Sources are returned in model feature order, preserving
// deterministic extraction regardless of worker count. The second return value
// is the count of source features that had at least one per-source acoustic
// override (any key listed in rls19AcousticOverrideKeys).
//
//nolint:gocognit,gocyclo,cyclop,funlen,maintidx // The override-merging rules are intentionally explicit and were preserved during extraction.
func extractRLS19RoadSources(model modelgeojson.Model, options rls19RoadRunOptions, supportedSourceTypes []string) ([]rls19road.RoadSource, int, error) {
	allowedSourceType := make(map[string]struct{}, len(supportedSourceTypes))
	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowedSourceType[trimmed] = struct{}{}
	}

	sources := make([]rls19road.RoadSource, 0)
	overrideCount := 0

	for featureIndex, feature := range model.Features {
		if feature.Kind != featureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, 0, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractRLS19RoadSources",
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		directionalSources, err := extractRLS19DirectionalSourceSpecs(feature)
		if err != nil {
			return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		err = validateRLS19DirectionalSurfaceTypes(feature, directionalSources, options.SurfaceType)
		if err != nil {
			return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("rls19-road-source-%03d", featureIndex)
		}

		if rls19FeatureHasAcousticOverrides(feature) {
			overrideCount++
		} else {
			for _, spec := range directionalSources {
				if rls19PropertiesHaveAcousticOverrides(spec.Overrides) {
					overrideCount++
					break
				}
			}
		}

		seenSourceIDs := make(map[string]struct{}, len(directionalSources))

		for lineIndex, directional := range directionalSources {
			sourceID := baseID
			if directional.IDHint != "" {
				sourceID = fmt.Sprintf("%s-%s", baseID, normalizeDirectionalSourceID(directional.IDHint, lineIndex))
			} else if len(directionalSources) > 1 {
				sourceID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

			if _, exists := seenSourceIDs[sourceID]; exists {
				return nil, 0, domainerrors.New(
					domainerrors.KindValidation,
					"cli.extractRLS19RoadSources",
					fmt.Sprintf("feature %q contains duplicate directional source id %q", feature.ID, sourceID),
					nil,
				)
			}

			seenSourceIDs[sourceID] = struct{}{}

			properties := mergedProperties(feature.Properties, directional.Overrides)

			surfaceType, err := resolveRLS19SurfaceType(properties, options.SurfaceType)
			if err != nil {
				return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			}

			laneCount, err := resolveRLS19LaneCount(properties)
			if err != nil {
				return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			}

			speedPkwKPH := options.SpeedPkwKPH
			speedLkw1KPH := options.SpeedLkw1KPH
			speedLkw2KPH := options.SpeedLkw2KPH
			speedKradKPH := options.SpeedKradKPH

			{
				value, ok, err := propertyFloat(properties, "road_speed_kph")
				if err != nil {
					return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					speedPkwKPH = value
					speedLkw1KPH = value
					speedLkw2KPH = value
					speedKradKPH = value
				}
			}

			for _, item := range []struct {
				keys   []string
				target *float64
			}{
				{[]string{"speed_pkw_kph"}, &speedPkwKPH},
				{[]string{"speed_lkw1_kph"}, &speedLkw1KPH},
				{[]string{"speed_lkw2_kph"}, &speedLkw2KPH},
				{[]string{"speed_krad_kph"}, &speedKradKPH},
			} {
				{
					value, ok, err := propertyFloat(properties, item.keys...)
					if err != nil {
						return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						*item.target = value
					}
				}
			}

			gradientPercent := options.GradientPercent

			{
				value, ok, err := propertyFloat(properties, "gradient_percent", "road_gradient_percent")
				if err != nil {
					return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					gradientPercent = value
				}
			}

			junctionDistanceM := 0.0

			{
				value, ok, err := propertyFloat(properties, "junction_distance_m", "road_junction_distance_m")
				if err != nil {
					return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					junctionDistanceM = value
				}
			}

			buildingHeightM := 0.0
			streetWidthM := 0.0

			{
				value, ok, err := propertyFloat(properties, "building_height_m")
				if err != nil {
					return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					buildingHeightM = value
				}
			}

			{
				value, ok, err := propertyFloat(properties, "street_width_m")
				if err != nil {
					return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					streetWidthM = value
				}
			}

			junctionType := rls19road.JunctionNone

			{
				value, ok, err := propertyString(properties, "junction_type", "road_junction_type")
				if err != nil {
					return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
				} else if ok {
					parsed, err := rls19road.ParseJunctionType(value)
					if err != nil {
						return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
					}

					junctionType = parsed
				}
			}

			trafficDay := rls19road.TrafficInput{
				PkwPerHour:  options.TrafficDayPkw,
				Lkw1PerHour: options.TrafficDayLkw1,
				Lkw2PerHour: options.TrafficDayLkw2,
				KradPerHour: options.TrafficDayKrad,
			}
			trafficNight := rls19road.TrafficInput{
				PkwPerHour:  options.TrafficNightPkw,
				Lkw1PerHour: options.TrafficNightLkw1,
				Lkw2PerHour: options.TrafficNightLkw2,
				KradPerHour: options.TrafficNightKrad,
			}

			for _, item := range []struct {
				keys   []string
				target *float64
			}{
				{[]string{"traffic_day_pkw"}, &trafficDay.PkwPerHour},
				{[]string{"traffic_day_lkw1"}, &trafficDay.Lkw1PerHour},
				{[]string{"traffic_day_lkw2"}, &trafficDay.Lkw2PerHour},
				{[]string{"traffic_day_krad"}, &trafficDay.KradPerHour},
				{[]string{"traffic_night_pkw"}, &trafficNight.PkwPerHour},
				{[]string{"traffic_night_lkw1"}, &trafficNight.Lkw1PerHour},
				{[]string{"traffic_night_lkw2"}, &trafficNight.Lkw2PerHour},
				{[]string{"traffic_night_krad"}, &trafficNight.KradPerHour},
			} {
				{
					value, ok, err := propertyFloat(properties, item.keys...)
					if err != nil {
						return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
					} else if ok {
						*item.target = value
					}
				}
			}

			source := rls19road.RoadSource{
				ID:                   sourceID,
				Centerline:           directional.Geometry.Centerline,
				CenterlineElevations: directional.Geometry.CenterlineElevations,
				LaneCount:            laneCount,
				SurfaceType:          rls19road.SurfaceType(surfaceType),
				Speeds: rls19road.SpeedInput{
					PkwKPH:  speedPkwKPH,
					Lkw1KPH: speedLkw1KPH,
					Lkw2KPH: speedLkw2KPH,
					KradKPH: speedKradKPH,
				},
				GradientPercent:   gradientPercent,
				JunctionType:      junctionType,
				JunctionDistanceM: junctionDistanceM,
				BuildingHeightM:   buildingHeightM,
				StreetWidthM:      streetWidthM,
				TrafficDay:        trafficDay,
				TrafficNight:      trafficNight,
			}

			err = source.Validate()
			if err != nil {
				return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", fmt.Sprintf("feature %q", feature.ID), err)
			}

			sources = append(sources, source)
		}
	}

	if len(sources) == 0 {
		return nil, 0, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19RoadSources", "model does not contain any supported line source features", nil)
	}

	return sources, overrideCount, nil
}

//nolint:nestif // The input decoding keeps the fallback precedence explicit for directional source specs.
func extractRLS19DirectionalSourceSpecs(feature modelgeojson.Feature) ([]rls19DirectionalSourceSpec, error) {
	rawDirectionalSources, ok := feature.Properties["rls19_directional_sources"]
	if ok && rawDirectionalSources != nil {
		items, ok := rawDirectionalSources.([]any)
		if !ok || len(items) == 0 {
			return nil, fmt.Errorf("property %q must be a non-empty array", "rls19_directional_sources")
		}

		specs := make([]rls19DirectionalSourceSpec, 0, len(items))
		for idx, item := range items {
			properties, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("property %q[%d] must be an object", "rls19_directional_sources", idx)
			}

			geometryValue, ok := properties["centerline"]
			if !ok || geometryValue == nil {
				if fallback, exists := properties["coordinates"]; exists && fallback != nil {
					geometryValue = fallback
				} else {
					return nil, fmt.Errorf("property %q[%d] requires centerline or coordinates", "rls19_directional_sources", idx)
				}
			}

			geometry, err := parseRLS19LineGeometry(geometryValue, properties)
			if err != nil {
				return nil, fmt.Errorf("property %q[%d]: %w", "rls19_directional_sources", idx, err)
			}

			idHint, _, err := propertyString(properties, "id", "direction_id", "direction")
			if err != nil {
				return nil, fmt.Errorf("property %q[%d]: %w", "rls19_directional_sources", idx, err)
			}

			specs = append(specs, rls19DirectionalSourceSpec{
				IDHint:    idHint,
				Geometry:  geometry,
				Overrides: properties,
			})
		}

		return specs, nil
	}

	geometries, err := rls19LineGeometriesFromFeature(feature)
	if err != nil {
		return nil, err
	}

	specs := make([]rls19DirectionalSourceSpec, 0, len(geometries))
	for _, geometry := range geometries {
		specs = append(specs, rls19DirectionalSourceSpec{Geometry: geometry})
	}

	return specs, nil
}

func rls19LineGeometriesFromFeature(feature modelgeojson.Feature) ([]rls19LineGeometry, error) {
	switch feature.GeometryType {
	case geometryTypeLineString:
		line, err := parseRLS19LineGeometry(feature.Coordinates, feature.Properties)
		if err != nil {
			return nil, err
		}

		return []rls19LineGeometry{line}, nil
	case geometryTypeMultiLineString:
		rawLines, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiLineString coordinates must be an array")
		}

		lines := make([]rls19LineGeometry, 0, len(rawLines))
		for _, rawLine := range rawLines {
			line, err := parseRLS19LineGeometry(rawLine, feature.Properties)
			if err != nil {
				return nil, err
			}

			lines = append(lines, line)
		}

		return lines, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (rls19-road supports LineString/MultiLineString only)", feature.GeometryType)
	}
}

func parseRLS19LineGeometry(value any, properties map[string]any) (rls19LineGeometry, error) {
	centerline, elevations, hasZ, err := parseLineStringCoordinates3D(value)
	if err != nil {
		return rls19LineGeometry{}, err
	}

	{
		propertyElevations, ok, err := propertyFloatSlice(properties, "centerline_elevations")
		if err != nil {
			return rls19LineGeometry{}, err
		} else if ok {
			if len(propertyElevations) != len(centerline) {
				return rls19LineGeometry{}, fmt.Errorf("centerline_elevations length %d must match centerline length %d", len(propertyElevations), len(centerline))
			}

			elevations = propertyElevations
			hasZ = true
		}
	}

	if !hasZ {
		{
			elevationM, ok, err := propertyFloat(properties, "elevation_m")
			if err != nil {
				return rls19LineGeometry{}, err
			} else if ok {
				elevations = make([]float64, len(centerline))
				for i := range elevations {
					elevations[i] = elevationM
				}

				hasZ = true
			}
		}
	}

	geometry := rls19LineGeometry{Centerline: centerline}
	if hasZ {
		geometry.CenterlineElevations = elevations
	}

	return geometry, nil
}

func normalizeDirectionalSourceID(raw string, fallbackIndex int) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Sprintf("%02d", fallbackIndex+1)
	}

	var builder strings.Builder

	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}

	normalized := strings.Trim(builder.String(), "-_.")
	if normalized == "" {
		return fmt.Sprintf("%02d", fallbackIndex+1)
	}

	return normalized
}

func extractRLS19Barriers(model modelgeojson.Model) ([]rls19road.Barrier, error) {
	barriers := make([]rls19road.Barrier, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "barrier" {
			continue
		}

		lines, err := lineStringsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Barriers", fmt.Sprintf("feature %q", feature.ID), err)
		}

		heightM, ok, err := featurePropertyFloat(feature, "height_m", "barrier_height_m")
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Barriers", fmt.Sprintf("feature %q", feature.ID), err)
		}

		if !ok {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Barriers", fmt.Sprintf("feature %q missing barrier height_m", feature.ID), nil)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("rls19-barrier-%03d", featureIndex)
		}

		for lineIndex, line := range lines {
			barrierID := baseID
			if len(lines) > 1 {
				barrierID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
			}

			barrier := rls19road.Barrier{
				ID:       barrierID,
				Geometry: line,
				HeightM:  heightM,
			}

			err := barrier.Validate()
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Barriers", fmt.Sprintf("feature %q", feature.ID), err)
			}

			barriers = append(barriers, barrier)
		}
	}

	return barriers, nil
}

func extractRLS19Buildings(model modelgeojson.Model) ([]rls19road.Building, error) {
	buildings := make([]rls19road.Building, 0)

	for featureIndex, feature := range model.Features {
		if feature.Kind != "building" {
			continue
		}

		polygons, err := polygonsFromFeature(feature)
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q", feature.ID), err)
		}

		heightM, ok, err := featurePropertyFloat(feature, "height_m", "building_height_m")
		if err != nil {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q", feature.ID), err)
		}

		if !ok {
			return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q missing building height_m", feature.ID), nil)
		}

		reflectionLossDB := 1.0

		{
			value, ok, err := featurePropertyFloat(feature, "reflection_loss_db")
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q", feature.ID), err)
			} else if ok {
				reflectionLossDB = value
			}
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("rls19-building-%03d", featureIndex)
		}

		for polygonIndex, polygon := range polygons {
			buildingID := baseID
			if len(polygons) > 1 {
				buildingID = fmt.Sprintf("%s-%02d", baseID, polygonIndex+1)
			}

			building := rls19road.Building{
				ID:               buildingID,
				Footprint:        polygon[0],
				HeightM:          heightM,
				ReflectionLossDB: reflectionLossDB,
			}

			err := building.Validate()
			if err != nil {
				return nil, domainerrors.New(domainerrors.KindValidation, "cli.extractRLS19Buildings", fmt.Sprintf("feature %q", feature.ID), err)
			}

			buildings = append(buildings, building)
		}
	}

	return buildings, nil
}
