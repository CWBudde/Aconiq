package cli

import (
	"fmt"
	"math"
	"strings"

	domainerrors "github.com/aconiq/backend/internal/domain/errors"
	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/standards/schall03"
)

// Property names carrying normative Schall 03 track data.  They are namespaced
// with the standard ID so they cannot be confused with the rail_* properties,
// which feed the preview data-pack path only.  See docs/geojson-schema-v1.md.
const (
	propSchall03Operations      = "schall03_operations"
	propSchall03Fahrbahn        = "schall03_fahrbahn"
	propSchall03SFahrbahn       = "schall03_s_fahrbahn"
	propSchall03Surface         = "schall03_surface"
	propSchall03BridgeType      = "schall03_bridge_type"
	propSchall03BridgeMitig     = "schall03_bridge_mitigation"
	propSchall03CurveRadius     = "schall03_curve_radius_m"
	propSchall03IsStation       = "schall03_is_station"
	propSchall03StreckeMaxKPH   = "schall03_strecke_max_kph"
	propSchall03WaterFraction   = "schall03_water_body_fraction"
	propSchall03PermanentlySlow = "schall03_permanently_slow"
	propSchall03TrackFeatures   = "schall03_track_features"

	propSchall03Reflective     = "schall03_reflective"
	propSchall03BaseHeight     = "schall03_base_height_m"
	propSchall03Thickness      = "schall03_thickness_m"
	propSchall03ParallelEdges  = "schall03_parallel_edges"
	propSchall03WallSurface    = "schall03_wall_surface"
	propSchall03ReflectingWall = "schall03_reflecting_wall"
)

const extractNormativeScope = "cli.extractSchall03NormativeScene"

// schall03NormativeScene is the normative input set the Anlage-2 chain
// consumes: track segments plus the reflecting and shielding geometry around
// them.
type schall03NormativeScene struct {
	Segments []schall03.TrackSegment
	Walls    []schall03.ReflectingWall
	Barriers []schall03.BarrierSegment
}

// modelHasSchall03NormativeTracks reports whether any source feature declares
// normative train operations.  This is the signal the `auto` engine setting
// resolves on: without operations there is no Zugart, no Fz composition and
// therefore no Beiblatt 1/2 spectrum to compute from.
func modelHasSchall03NormativeTracks(model modelgeojson.Model) bool {
	for _, feature := range model.Features {
		if feature.Kind != modelgeojson.FeatureKindSource {
			continue
		}

		if raw, ok := feature.Properties[propSchall03Operations]; ok && raw != nil {
			return true
		}
	}

	return false
}

// extractSchall03NormativeScene converts a normalized model into the normative
// Schall 03 input set.
func extractSchall03NormativeScene(model modelgeojson.Model, supportedSourceTypes []string) (schall03NormativeScene, error) {
	allowedSourceType := normalizedSourceTypeSet(supportedSourceTypes)

	var scene schall03NormativeScene

	for featureIndex, feature := range model.Features {
		var err error

		switch feature.Kind {
		case modelgeojson.FeatureKindSource:
			err = appendSchall03Segments(&scene, feature, featureIndex, allowedSourceType)
		case modelgeojson.FeatureKindBarrier:
			err = appendSchall03Barriers(&scene, feature, featureIndex)
		case modelgeojson.FeatureKindBuilding:
			err = appendSchall03Building(&scene, feature, featureIndex)
		}

		if err != nil {
			return schall03NormativeScene{}, err
		}
	}

	if len(scene.Segments) == 0 {
		return schall03NormativeScene{}, domainerrors.New(
			domainerrors.KindValidation,
			extractNormativeScope,
			"model does not contain any line source feature carrying "+propSchall03Operations,
			nil,
		)
	}

	return scene, nil
}

func normalizedSourceTypeSet(supportedSourceTypes []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(supportedSourceTypes))

	for _, sourceType := range supportedSourceTypes {
		trimmed := strings.ToLower(strings.TrimSpace(sourceType))
		if trimmed == "" {
			continue
		}

		allowed[trimmed] = struct{}{}
	}

	return allowed
}

func appendSchall03Segments(
	scene *schall03NormativeScene,
	feature modelgeojson.Feature,
	featureIndex int,
	allowedSourceType map[string]struct{},
) error {
	rawOperations, ok := feature.Properties[propSchall03Operations]
	if !ok || rawOperations == nil {
		return nil
	}

	normalizedSourceType := strings.ToLower(strings.TrimSpace(feature.SourceType))
	if normalizedSourceType != "" {
		if _, supported := allowedSourceType[normalizedSourceType]; !supported {
			return validationErrorf("feature %q source_type %q is not supported by selected standard/profile", feature.ID, feature.SourceType)
		}
	}

	operations, err := parseSchall03Operations(feature.ID, rawOperations)
	if err != nil {
		return err
	}

	template, err := parseSchall03SegmentTemplate(feature)
	if err != nil {
		return err
	}

	lines, err := lineStringsFromFeature(feature, schall03.StandardID)
	if err != nil {
		return domainerrors.New(domainerrors.KindValidation, extractNormativeScope, fmt.Sprintf("feature %q", feature.ID), err)
	}

	baseID := strings.TrimSpace(feature.ID)
	if baseID == "" {
		baseID = fmt.Sprintf("schall03-segment-%03d", featureIndex)
	}

	for lineIndex, line := range lines {
		segment := template
		segment.ID = baseID

		if len(lines) > 1 {
			segment.ID = fmt.Sprintf("%s-%02d", baseID, lineIndex+1)
		}

		segment.TrackCenterline = line
		segment.Operations = operations

		validateErr := segment.Validate()
		if validateErr != nil {
			return domainerrors.New(domainerrors.KindValidation, extractNormativeScope, fmt.Sprintf("feature %q", feature.ID), validateErr)
		}

		scene.Segments = append(scene.Segments, segment)
	}

	return nil
}

// parseSchall03SegmentTemplate reads every per-track property except geometry
// and operations, which vary per line part.
func parseSchall03SegmentTemplate(feature modelgeojson.Feature) (schall03.TrackSegment, error) {
	var segment schall03.TrackSegment

	err := parseSchall03TrackEnums(feature, &segment)
	if err != nil {
		return segment, err
	}

	err = parseSchall03TrackNumbers(feature, &segment)
	if err != nil {
		return segment, err
	}

	err = parseSchall03TrackFlags(feature, &segment)
	if err != nil {
		return segment, err
	}

	return segment, parseSchall03TrackFeatures(feature, &segment)
}

// parseSchall03TrackFeatures decodes the optional schall03_track_features array
// — the Weichen, Kreuzungen and Haltestellen that Nr. 5.3.2 scopes the 50 km/h
// substitute speed to.  A track that declares none keeps the whole-segment
// reading.
func parseSchall03TrackFeatures(feature modelgeojson.Feature, segment *schall03.TrackSegment) error {
	raw, ok := feature.Properties[propSchall03TrackFeatures]
	if !ok || raw == nil {
		return nil
	}

	entries, entriesOK := raw.([]any)
	if !entriesOK {
		return validationErrorf("feature %q property %q must be an array", feature.ID, propSchall03TrackFeatures)
	}

	features := make([]schall03.TrackFeature, 0, len(entries))

	for index, entry := range entries {
		object, objectOK := entry.(map[string]any)
		if !objectOK {
			return validationErrorf(
				"feature %q property %q[%d] must be an object",
				feature.ID, propSchall03TrackFeatures, index,
			)
		}

		parsed, err := parseSchall03TrackFeature(feature.ID, index, object)
		if err != nil {
			return err
		}

		features = append(features, parsed)
	}

	segment.Features = features

	return nil
}

func parseSchall03TrackFeature(featureID string, index int, object map[string]any) (schall03.TrackFeature, error) {
	where := fmt.Sprintf("feature %q property %q[%d]", featureID, propSchall03TrackFeatures, index)

	kind, _, err := propertyString(object, "kind")
	if err != nil {
		return schall03.TrackFeature{}, validationErrorf("%s: %v", where, err)
	}

	x, _, err := propertyFloat(object, "x")
	if err != nil {
		return schall03.TrackFeature{}, validationErrorf("%s: %v", where, err)
	}

	y, _, err := propertyFloat(object, "y")
	if err != nil {
		return schall03.TrackFeature{}, validationErrorf("%s: %v", where, err)
	}

	parsed := schall03.TrackFeature{
		Kind:  schall03.TrackFeatureKind(kind),
		Point: geo.Point2D{X: x, Y: y},
	}

	err = parsed.Validate()
	if err != nil {
		return schall03.TrackFeature{}, validationErrorf("%s: %v", where, err)
	}

	return parsed, nil
}

// parseSchall03TrackEnums resolves the Tabelle 7 / 15 / 8 vocabularies.
func parseSchall03TrackEnums(feature modelgeojson.Feature, segment *schall03.TrackSegment) error {
	fahrbahnName, _, err := featurePropertyString(feature, propSchall03Fahrbahn)
	if err != nil {
		return propertyError(feature.ID, propSchall03Fahrbahn, err)
	}

	segment.Fahrbahn, err = schall03.ParseFahrbahnart(fahrbahnName)
	if err != nil {
		return propertyError(feature.ID, propSchall03Fahrbahn, err)
	}

	sFahrbahnName, _, err := featurePropertyString(feature, propSchall03SFahrbahn)
	if err != nil {
		return propertyError(feature.ID, propSchall03SFahrbahn, err)
	}

	segment.SFahrbahn, err = schall03.ParseSFahrbahnart(sFahrbahnName)
	if err != nil {
		return propertyError(feature.ID, propSchall03SFahrbahn, err)
	}

	surfaceName, _, err := featurePropertyString(feature, propSchall03Surface)
	if err != nil {
		return propertyError(feature.ID, propSchall03Surface, err)
	}

	segment.Surface, err = schall03.ParseSurfaceCond(surfaceName)
	if err != nil {
		return propertyError(feature.ID, propSchall03Surface, err)
	}

	return nil
}

func parseSchall03TrackNumbers(feature modelgeojson.Feature, segment *schall03.TrackSegment) error {
	bridgeType, _, err := featurePropertyFloat(feature, propSchall03BridgeType)
	if err != nil {
		return propertyError(feature.ID, propSchall03BridgeType, err)
	}

	if bridgeType != math.Trunc(bridgeType) {
		return validationErrorf("feature %q property %q must be a whole number 0-4", feature.ID, propSchall03BridgeType)
	}

	segment.BridgeType = int(bridgeType)

	for _, field := range []struct {
		key    string
		target *float64
	}{
		{"elevation_m", &segment.ElevationM},
		{propSchall03CurveRadius, &segment.CurveRadiusM},
		{propSchall03StreckeMaxKPH, &segment.StreckeMaxKPH},
		{propSchall03WaterFraction, &segment.WaterBodyFractionW},
	} {
		value, ok, floatErr := featurePropertyFloat(feature, field.key)
		if floatErr != nil {
			return propertyError(feature.ID, field.key, floatErr)
		}

		if ok {
			*field.target = value
		}
	}

	if segment.StreckeMaxKPH == 0 {
		return validationErrorf("feature %q requires property %q (Streckenhöchstgeschwindigkeit in km/h)", feature.ID, propSchall03StreckeMaxKPH)
	}

	return nil
}

func parseSchall03TrackFlags(feature modelgeojson.Feature, segment *schall03.TrackSegment) error {
	for _, field := range []struct {
		key    string
		target *bool
	}{
		{propSchall03BridgeMitig, &segment.BridgeMitig},
		{propSchall03IsStation, &segment.IsStation},
		{propSchall03PermanentlySlow, &segment.PermanentlySlow},
	} {
		value, ok, err := featurePropertyBool(feature, field.key)
		if err != nil {
			return propertyError(feature.ID, field.key, err)
		}

		if ok {
			*field.target = value
		}
	}

	return nil
}

// parseSchall03Operations decodes the schall03_operations array.  Each entry
// either names a Zugart from Beiblatt 1/2 or gives an explicit Fz composition.
func parseSchall03Operations(featureID string, raw any) ([]schall03.TrainOperation, error) {
	entries, ok := raw.([]any)
	if !ok || len(entries) == 0 {
		return nil, validationErrorf("feature %q property %q must be a non-empty array", featureID, propSchall03Operations)
	}

	operations := make([]schall03.TrainOperation, 0, len(entries))

	for index, entry := range entries {
		object, objectOK := entry.(map[string]any)
		if !objectOK {
			return nil, validationErrorf("feature %q property %q[%d] must be an object", featureID, propSchall03Operations, index)
		}

		operation, err := parseSchall03Operation(featureID, index, object)
		if err != nil {
			return nil, err
		}

		operations = append(operations, operation)
	}

	return operations, nil
}

func parseSchall03Operation(featureID string, index int, object map[string]any) (schall03.TrainOperation, error) {
	where := fmt.Sprintf("feature %q property %q[%d]", featureID, propSchall03Operations, index)

	trainsDay, _, err := propertyFloat(object, "trains_per_hour_day")
	if err != nil {
		return schall03.TrainOperation{}, validationErrorf("%s: %v", where, err)
	}

	trainsNight, _, err := propertyFloat(object, "trains_per_hour_night")
	if err != nil {
		return schall03.TrainOperation{}, validationErrorf("%s: %v", where, err)
	}

	zugart, hasZugart, err := propertyString(object, "zugart")
	if err != nil {
		return schall03.TrainOperation{}, validationErrorf("%s: %v", where, err)
	}

	var operation schall03.TrainOperation

	switch {
	case hasZugart:
		fromZugart, zugartErr := schall03.NewTrainOperationFromZugart(zugart, trainsDay, trainsNight)
		if zugartErr != nil {
			return schall03.TrainOperation{}, validationErrorf("%s: %v, expected one of %s", where, zugartErr, strings.Join(schall03.ZugartNames(), ", "))
		}

		operation = *fromZugart
	default:
		composition, compErr := parseSchall03FzComposition(where, object)
		if compErr != nil {
			return schall03.TrainOperation{}, compErr
		}

		operation = schall03.TrainOperation{
			TrainType:          "custom",
			FzComposition:      composition,
			TrainsPerHourDay:   trainsDay,
			TrainsPerHourNight: trainsNight,
		}
	}

	trainType, hasTrainType, err := propertyString(object, "train_type")
	if err != nil {
		return schall03.TrainOperation{}, validationErrorf("%s: %v", where, err)
	}

	if hasTrainType {
		operation.TrainType = trainType
	}

	speed, hasSpeed, err := propertyFloat(object, "speed_kph")
	if err != nil {
		return schall03.TrainOperation{}, validationErrorf("%s: %v", where, err)
	}

	if hasSpeed {
		operation.SpeedKPH = speed
	}

	validateErr := operation.Validate()
	if validateErr != nil {
		return schall03.TrainOperation{}, validationErrorf("%s: %v", where, validateErr)
	}

	return operation, nil
}

func parseSchall03FzComposition(where string, object map[string]any) ([]schall03.FzCount, error) {
	raw, ok := object["fz_composition"]
	if !ok || raw == nil {
		return nil, validationErrorf("%s requires either \"zugart\" or \"fz_composition\"", where)
	}

	entries, entriesOK := raw.([]any)
	if !entriesOK || len(entries) == 0 {
		return nil, validationErrorf("%s: \"fz_composition\" must be a non-empty array", where)
	}

	composition := make([]schall03.FzCount, 0, len(entries))

	for index, entry := range entries {
		object, objectOK := entry.(map[string]any)
		if !objectOK {
			return nil, validationErrorf("%s: \"fz_composition\"[%d] must be an object", where, index)
		}

		fz, _, err := propertyFloat(object, "fz")
		if err != nil {
			return nil, validationErrorf("%s: \"fz_composition\"[%d]: %v", where, index, err)
		}

		count, _, err := propertyFloat(object, "count")
		if err != nil {
			return nil, validationErrorf("%s: \"fz_composition\"[%d]: %v", where, index, err)
		}

		if fz != math.Trunc(fz) || count != math.Trunc(count) {
			return nil, validationErrorf("%s: \"fz_composition\"[%d] fz and count must be whole numbers", where, index)
		}

		composition = append(composition, schall03.FzCount{Fz: int(fz), Count: int(count)})
	}

	return composition, nil
}

// appendSchall03Barriers converts one barrier feature into BarrierSegments.
// Every consecutive vertex pair becomes one straight barrier panel; a barrier
// marked reflective additionally becomes a ReflectingWall, because Gl. 19-20
// treats the two effects separately.
//
// Every panel of one polyline carries that polyline's ObstacleID, so lateral
// diffraction (Nr. 6.5) rounds only the polyline's free ends.  Without it an
// interior vertex of a three-point barrier is a Seitenkante, and since
// barrierDz is 10·lg(3 + …) any z > 0 however small caps the whole wall at
// about 4.8 dB per band.  The ReflectingWall a reflective barrier emits carries
// the same ObstacleID, so the panel does not diffract the reflection it just
// produced.
func appendSchall03Barriers(scene *schall03NormativeScene, feature modelgeojson.Feature, featureIndex int) error {
	if feature.HeightM == nil || *feature.HeightM <= 0 {
		return validationErrorf("barrier feature %q requires height_m > 0", feature.ID)
	}

	lines, err := lineStringsFromFeature(feature, schall03.StandardID)
	if err != nil {
		return domainerrors.New(domainerrors.KindValidation, extractNormativeScope, fmt.Sprintf("barrier feature %q", feature.ID), err)
	}

	reflective, _, err := featurePropertyBool(feature, propSchall03Reflective)
	if err != nil {
		return propertyError(feature.ID, propSchall03Reflective, err)
	}

	parallelEdges, _, err := featurePropertyBool(feature, propSchall03ParallelEdges)
	if err != nil {
		return propertyError(feature.ID, propSchall03ParallelEdges, err)
	}

	baseHeightM, _, err := featurePropertyFloat(feature, propSchall03BaseHeight)
	if err != nil {
		return propertyError(feature.ID, propSchall03BaseHeight, err)
	}

	thicknessM, _, err := featurePropertyFloat(feature, propSchall03Thickness)
	if err != nil {
		return propertyError(feature.ID, propSchall03Thickness, err)
	}

	surface, err := featureWallSurface(feature)
	if err != nil {
		return err
	}

	for lineIndex, line := range lines {
		obstacleID := schall03ObstaclePartID(feature, featureIndex, lineIndex)

		for i := range len(line) - 1 {
			barrier := schall03.BarrierSegment{
				A:           line[i],
				B:           line[i+1],
				TopHeightM:  *feature.HeightM,
				BaseHeightM: baseHeightM,
				Reflective:  reflective,
				ThicknessM:  thicknessM,
				IsParallel:  parallelEdges,
				ObstacleID:  obstacleID,
			}

			validateErr := barrier.Validate()
			if validateErr != nil {
				return domainerrors.New(domainerrors.KindValidation, extractNormativeScope, fmt.Sprintf("barrier feature %q", feature.ID), validateErr)
			}

			scene.Barriers = append(scene.Barriers, barrier)

			if !reflective {
				continue
			}

			err = appendReflectingWall(scene, feature.ID, obstacleID, line[i], line[i+1], *feature.HeightM, surface)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// appendSchall03Building turns a building footprint into shielding obstacles
// and, on request, into reflecting walls.
//
// Shielding is unconditional: a receiver behind a house is behind a house, and
// height_m is already mandatory on a building feature, so no model that
// validates today starts failing.  Reflection stays opt-in, because it raises
// levels for every model that never asked for it and because reflection paths
// are enumerated to MaxReflectionOrder over every facade.
//
// The panels of one ring share an ObstacleID: a lateral path may round the
// footprint's outermost silhouette vertex, never the end of a single wall
// panel, which would run straight through the building.  The ring's
// ReflectingWalls carry that same ObstacleID, because a facade is obstacle and
// reflector at once and a mirrored ray crosses its own reflector by
// construction — without the shared identity, opting a building into reflection
// would make its own footprint shield the reflection off it.  Reflective is
// left false — Gl. 20's D_refl is scoped to reflektierende Schallschutzwände
// mit absorbierendem Sockel, and a house is not a Schallschutzwand.
func appendSchall03Building(scene *schall03NormativeScene, feature modelgeojson.Feature, featureIndex int) error {
	reflecting, _, err := featurePropertyBool(feature, propSchall03ReflectingWall)
	if err != nil {
		return propertyError(feature.ID, propSchall03ReflectingWall, err)
	}

	if feature.HeightM == nil || *feature.HeightM <= 0 {
		return validationErrorf("building feature %q requires height_m > 0", feature.ID)
	}

	surface, err := featureWallSurface(feature)
	if err != nil {
		return err
	}

	polygons, err := polygonsFromFeature(feature, schall03.StandardID)
	if err != nil {
		return domainerrors.New(domainerrors.KindValidation, extractNormativeScope, fmt.Sprintf("building feature %q", feature.ID), err)
	}

	for polygonIndex, polygon := range polygons {
		if len(polygon) == 0 {
			continue
		}

		obstacleID := schall03ObstaclePartID(feature, featureIndex, polygonIndex)

		// Only the outer ring shields and reflects towards the track; inner
		// rings are courtyards and cannot see a source outside the footprint.
		ring := polygon[0]
		for i := range len(ring) - 1 {
			barrier := schall03.BarrierSegment{
				A:          ring[i],
				B:          ring[i+1],
				TopHeightM: *feature.HeightM,
				ObstacleID: obstacleID,
			}

			validateErr := barrier.Validate()
			if validateErr != nil {
				return domainerrors.New(domainerrors.KindValidation, extractNormativeScope, fmt.Sprintf("building feature %q", feature.ID), validateErr)
			}

			scene.Barriers = append(scene.Barriers, barrier)

			if !reflecting {
				continue
			}

			err = appendReflectingWall(scene, feature.ID, obstacleID, ring[i], ring[i+1], *feature.HeightM, surface)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// schall03ObstaclePartID names one part of one feature's geometry.
//
// The identity is internal: it never leaves the extracted scene, it is not part
// of the GeoJSON vocabulary, and nothing serialises it into a run artifact.  It
// therefore does not have to read like the feature's id, and must not be
// derivable from it alone — a `MultiPolygon` named `house` and a separate
// polygon named `house-01` are both legal, model validation only checks the
// original ids, and merging their panels into one obstacle would silently move
// numbers: the lateral candidate set, the group's top height and the
// coincident-crossing dedupe all read the group as one building.
//
// The feature's kind and its index in the model come first and pin the identity
// to exactly one feature, whatever it or any other feature is called; the part
// index separates the rings of one multipart geometry.  The trailing id is for
// the human reading a scene dump and carries no weight.
func schall03ObstaclePartID(feature modelgeojson.Feature, featureIndex, partIndex int) string {
	return fmt.Sprintf("%s#%d#%d#%s", feature.Kind, featureIndex, partIndex, strings.TrimSpace(feature.ID))
}

func appendReflectingWall(
	scene *schall03NormativeScene,
	featureID string,
	obstacleID string,
	a, b geo.Point2D,
	heightM float64,
	surface schall03.WallSurfaceType,
) error {
	wall := schall03.ReflectingWall{A: a, B: b, HeightM: heightM, Surface: surface, ObstacleID: obstacleID}

	err := wall.Validate()
	if err != nil {
		return domainerrors.New(domainerrors.KindValidation, extractNormativeScope, fmt.Sprintf("feature %q", featureID), err)
	}

	scene.Walls = append(scene.Walls, wall)

	return nil
}

func featureWallSurface(feature modelgeojson.Feature) (schall03.WallSurfaceType, error) {
	name, _, err := featurePropertyString(feature, propSchall03WallSurface)
	if err != nil {
		return 0, propertyError(feature.ID, propSchall03WallSurface, err)
	}

	if strings.TrimSpace(name) == "" && feature.Kind == modelgeojson.FeatureKindBuilding {
		// Tabelle 18 row "Gebäudewände mit Fenstern und kleinen Anbauten".
		name = "building"
	}

	surface, err := schall03.ParseWallSurface(name)
	if err != nil {
		return 0, propertyError(feature.ID, propSchall03WallSurface, err)
	}

	return surface, nil
}

func propertyError(featureID, key string, err error) error {
	return domainerrors.New(
		domainerrors.KindValidation,
		extractNormativeScope,
		fmt.Sprintf("feature %q property %q", featureID, key),
		err,
	)
}

func validationErrorf(format string, args ...any) error {
	return domainerrors.New(domainerrors.KindValidation, extractNormativeScope, fmt.Sprintf(format, args...), nil)
}
