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
	runParams.SurfaceType.key(), runParams.RoadSurfaceType.key(),
	runParams.RoadSpeedKPH.key(), runParams.SpeedPkwKPH.key(), runParams.SpeedLkw1KPH.key(), runParams.SpeedLkw2KPH.key(), runParams.SpeedKradKPH.key(),
	runParams.GradientPercent.key(), runParams.RoadGradientPercent.key(),
	runParams.JunctionType.key(), runParams.RoadJunctionType.key(),
	runParams.JunctionDistanceM.key(), runParams.RoadJunctionDistanceM.key(),
	runParams.BuildingHeightM.key(),
	runParams.StreetWidthM.key(),
	runParams.TrafficDayPkw.key(), runParams.TrafficDayLkw1.key(), runParams.TrafficDayLkw2.key(), runParams.TrafficDayKrad.key(),
	runParams.TrafficNightPkw.key(), runParams.TrafficNightLkw1.key(), runParams.TrafficNightLkw2.key(), runParams.TrafficNightKrad.key(),
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

	value, ok, err := propertyString(properties, runParams.SurfaceType.key(), runParams.RoadSurfaceType.key())
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
const rls19RoadScope = "cli.extractRLS19RoadSources"

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
		if feature.Kind != modelgeojson.FeatureKindSource {
			continue
		}

		normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
		if normalizedSourceType != "" {
			if _, ok := allowedSourceType[normalizedSourceType]; !ok {
				return nil, 0, domainerrors.New(
					domainerrors.KindValidation,
					rls19RoadScope,
					fmt.Sprintf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType),
					nil,
				)
			}
		}

		// An area source feature is a Parkplatz (§3.4), which
		// extractRLS19ParkingSources consumes. The guard above has already
		// refused any source_type that is neither line nor area.
		if normalizedSourceType == modelgeojson.SourceTypeArea {
			continue
		}

		directionalSources, err := extractRLS19DirectionalSourceSpecs(feature)
		if err != nil {
			return nil, 0, domainerrors.New(domainerrors.KindValidation, rls19RoadScope, fmt.Sprintf("feature %q", feature.ID), err)
		}

		err = validateRLS19DirectionalSurfaceTypes(feature, directionalSources, options.SurfaceType)
		if err != nil {
			return nil, 0, domainerrors.New(domainerrors.KindValidation, rls19RoadScope, fmt.Sprintf("feature %q", feature.ID), err)
		}

		baseID := strings.TrimSpace(feature.ID)
		if baseID == "" {
			baseID = fmt.Sprintf("rls19-road-source-%03d", featureIndex)
		}

		if rls19FeatureCarriesAcousticOverride(feature, directionalSources) {
			overrideCount++
		}

		seenSourceIDs := make(map[string]struct{}, len(directionalSources))

		for lineIndex, directional := range directionalSources {
			sourceID := rls19DirectionalSourceID(baseID, directional, lineIndex, len(directionalSources))

			if _, exists := seenSourceIDs[sourceID]; exists {
				return nil, 0, domainerrors.New(
					domainerrors.KindValidation,
					rls19RoadScope,
					fmt.Sprintf("feature %q contains duplicate directional source id %q", feature.ID, sourceID),
					nil,
				)
			}

			seenSourceIDs[sourceID] = struct{}{}

			source, buildErr := buildRLS19RoadSource(feature, options, sourceID, directional)
			if buildErr != nil {
				return nil, 0, buildErr
			}

			sources = append(sources, source)
		}
	}

	// An empty result is not an error here: a model may carry only Parkplatz
	// features. Only the runner can see both source kinds, so it owns the
	// decision that a model carries no rls19-road source at all.
	return sources, overrideCount, nil
}

// extractRLS19DirectionalSourceSpecs reads the explicit rls19_directional_sources
// array when the feature carries one, and otherwise derives one spec per line
// geometry. The fallback precedence is unchanged; it is now expressed as an
// early return rather than as nesting.
func extractRLS19DirectionalSourceSpecs(feature modelgeojson.Feature) ([]rls19DirectionalSourceSpec, error) {
	rawDirectionalSources, ok := feature.Properties["rls19_directional_sources"]
	if !ok || rawDirectionalSources == nil {
		return rls19FallbackDirectionalSourceSpecs(feature)
	}

	items, ok := rawDirectionalSources.([]any)
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("property %q must be a non-empty array", "rls19_directional_sources")
	}

	specs := make([]rls19DirectionalSourceSpec, 0, len(items))

	for idx, item := range items {
		spec, err := parseRLS19DirectionalSourceSpec(item, idx)
		if err != nil {
			return nil, err
		}

		specs = append(specs, spec)
	}

	return specs, nil
}

// parseRLS19DirectionalSourceSpec decodes one entry of the
// rls19_directional_sources array. idx only appears in the error messages.
func parseRLS19DirectionalSourceSpec(item any, idx int) (rls19DirectionalSourceSpec, error) {
	properties, ok := item.(map[string]any)
	if !ok {
		return rls19DirectionalSourceSpec{}, fmt.Errorf("property %q[%d] must be an object", "rls19_directional_sources", idx)
	}

	geometryValue, ok := properties["centerline"]
	if !ok || geometryValue == nil {
		fallback, exists := properties["coordinates"]
		if !exists || fallback == nil {
			return rls19DirectionalSourceSpec{}, fmt.Errorf("property %q[%d] requires centerline or coordinates", "rls19_directional_sources", idx)
		}

		geometryValue = fallback
	}

	geometry, err := parseRLS19LineGeometry(geometryValue, properties)
	if err != nil {
		return rls19DirectionalSourceSpec{}, fmt.Errorf("property %q[%d]: %w", "rls19_directional_sources", idx, err)
	}

	idHint, _, err := propertyString(properties, "id", "direction_id", "direction")
	if err != nil {
		return rls19DirectionalSourceSpec{}, fmt.Errorf("property %q[%d]: %w", "rls19_directional_sources", idx, err)
	}

	return rls19DirectionalSourceSpec{
		IDHint:    idHint,
		Geometry:  geometry,
		Overrides: properties,
	}, nil
}

// rls19FallbackDirectionalSourceSpecs derives one spec per line geometry for a
// feature that declares no explicit directional sources.
func rls19FallbackDirectionalSourceSpecs(feature modelgeojson.Feature) ([]rls19DirectionalSourceSpec, error) {
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
	case modelgeojson.GeometryTypeLineString:
		line, err := parseRLS19LineGeometry(feature.Coordinates, feature.Properties)
		if err != nil {
			return nil, err
		}

		return []rls19LineGeometry{line}, nil
	case modelgeojson.GeometryTypeMultiLineString:
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
		if feature.Kind != modelgeojson.FeatureKindBarrier {
			continue
		}

		lines, err := lineStringsFromFeature(feature, rls19road.StandardID)
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
		if feature.Kind != modelgeojson.FeatureKindBuilding {
			continue
		}

		polygons, err := polygonsFromFeature(feature, rls19road.StandardID)
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

		// RLS-19 Tabelle 8, facade row: buildings reflect with D_RV = 0.5 dB
		// unless the feature states otherwise.
		reflectionLossDB := 0.5

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

// buildRLS19RoadSource merges the run options with one feature's properties and
// one directional source's overrides into a single road source.
//
// The table order is behaviour: road_speed_kph fans out to all four vehicle
// classes and must precede the per-class keys that refine it, and junction_type
// is parsed inside the table so a value the parser rejects is reported before
// the traffic keys rather than after them.
func buildRLS19RoadSource(feature modelgeojson.Feature, options rls19RoadRunOptions, sourceID string, directional rls19DirectionalSourceSpec) (rls19road.RoadSource, error) {
	properties := mergedProperties(feature.Properties, directional.Overrides)

	surfaceType, err := resolveRLS19SurfaceType(properties, options.SurfaceType)
	if err != nil {
		return rls19road.RoadSource{}, rls19RoadFeatureError(feature, err)
	}

	laneCount, err := resolveRLS19LaneCount(properties)
	if err != nil {
		return rls19road.RoadSource{}, rls19RoadFeatureError(feature, err)
	}

	source := rls19road.RoadSource{
		ID:                   sourceID,
		Centerline:           directional.Geometry.Centerline,
		CenterlineElevations: directional.Geometry.CenterlineElevations,
		LaneCount:            laneCount,
		SurfaceType:          rls19road.SurfaceType(surfaceType),
		Speeds: rls19road.SpeedInput{
			PkwKPH:  options.SpeedPkwKPH,
			Lkw1KPH: options.SpeedLkw1KPH,
			Lkw2KPH: options.SpeedLkw2KPH,
			KradKPH: options.SpeedKradKPH,
		},
		GradientPercent:   options.GradientPercent,
		JunctionType:      rls19road.JunctionNone,
		JunctionDistanceM: 0,
		BuildingHeightM:   0,
		StreetWidthM:      0,
		TrafficDay: rls19road.TrafficInput{
			PkwPerHour:  options.TrafficDayPkw,
			Lkw1PerHour: options.TrafficDayLkw1,
			Lkw2PerHour: options.TrafficDayLkw2,
			KradPerHour: options.TrafficDayKrad,
		},
		TrafficNight: rls19road.TrafficInput{
			PkwPerHour:  options.TrafficNightPkw,
			Lkw1PerHour: options.TrafficNightLkw1,
			Lkw2PerHour: options.TrafficNightLkw2,
			KradPerHour: options.TrafficNightKrad,
		},
	}

	overrideErr := applyPropertyOverrides(properties, rls19RoadScope, feature.ID, []propertyOverride{
		overrideFloats([]*float64{
			&source.Speeds.PkwKPH,
			&source.Speeds.Lkw1KPH,
			&source.Speeds.Lkw2KPH,
			&source.Speeds.KradKPH,
		}, runParams.RoadSpeedKPH.key()),
		overrideFloat(&source.Speeds.PkwKPH, runParams.SpeedPkwKPH.key()),
		overrideFloat(&source.Speeds.Lkw1KPH, runParams.SpeedLkw1KPH.key()),
		overrideFloat(&source.Speeds.Lkw2KPH, runParams.SpeedLkw2KPH.key()),
		overrideFloat(&source.Speeds.KradKPH, runParams.SpeedKradKPH.key()),
		overrideFloat(&source.GradientPercent, runParams.GradientPercent.key(), runParams.RoadGradientPercent.key()),
		overrideFloat(&source.JunctionDistanceM, runParams.JunctionDistanceM.key(), runParams.RoadJunctionDistanceM.key()),
		overrideFloat(&source.BuildingHeightM, runParams.BuildingHeightM.key()),
		overrideFloat(&source.StreetWidthM, runParams.StreetWidthM.key()),
		overrideParsed(&source.JunctionType, rls19road.ParseJunctionType, runParams.JunctionType.key(), runParams.RoadJunctionType.key()),
		overrideFloat(&source.TrafficDay.PkwPerHour, runParams.TrafficDayPkw.key()),
		overrideFloat(&source.TrafficDay.Lkw1PerHour, runParams.TrafficDayLkw1.key()),
		overrideFloat(&source.TrafficDay.Lkw2PerHour, runParams.TrafficDayLkw2.key()),
		overrideFloat(&source.TrafficDay.KradPerHour, runParams.TrafficDayKrad.key()),
		overrideFloat(&source.TrafficNight.PkwPerHour, runParams.TrafficNightPkw.key()),
		overrideFloat(&source.TrafficNight.Lkw1PerHour, runParams.TrafficNightLkw1.key()),
		overrideFloat(&source.TrafficNight.Lkw2PerHour, runParams.TrafficNightLkw2.key()),
		overrideFloat(&source.TrafficNight.KradPerHour, runParams.TrafficNightKrad.key()),
	})
	if overrideErr != nil {
		return rls19road.RoadSource{}, overrideErr
	}

	err = source.Validate()
	if err != nil {
		return rls19road.RoadSource{}, rls19RoadFeatureError(feature, err)
	}

	return source, nil
}

// rls19RoadFeatureError is the wrapping every failure in this extractor uses.
func rls19RoadFeatureError(feature modelgeojson.Feature, err error) error {
	return domainerrors.New(domainerrors.KindValidation, rls19RoadScope, fmt.Sprintf("feature %q", feature.ID), err)
}

// rls19FeatureCarriesAcousticOverride reports whether a feature contributes to
// the per-source override count: either it carries an acoustic key itself, or
// at least one of its directional sources does. A feature counts once however
// many of its directional sources match.
func rls19FeatureCarriesAcousticOverride(feature modelgeojson.Feature, directionalSources []rls19DirectionalSourceSpec) bool {
	if rls19FeatureHasAcousticOverrides(feature) {
		return true
	}

	for _, spec := range directionalSources {
		if rls19PropertiesHaveAcousticOverrides(spec.Overrides) {
			return true
		}
	}

	return false
}

// rls19DirectionalSourceID names one directional source. An explicit id hint
// wins; otherwise a feature emitting more than one source suffixes by position,
// and a feature emitting exactly one keeps the feature's own ID.
func rls19DirectionalSourceID(baseID string, directional rls19DirectionalSourceSpec, lineIndex, total int) string {
	if directional.IDHint != "" {
		return fmt.Sprintf("%s-%s", baseID, normalizeDirectionalSourceID(directional.IDHint, lineIndex))
	}

	if total > 1 {
		return fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
	}

	return baseID
}
