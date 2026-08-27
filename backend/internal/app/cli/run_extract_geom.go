package cli

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

func sourcePointsFromFeature(feature modelgeojson.Feature) ([]geo.Point2D, error) {
	switch feature.GeometryType {
	case "Point":
		point, err := parsePointCoordinate(feature.Coordinates)
		if err != nil {
			return nil, err
		}

		return []geo.Point2D{point}, nil
	case "MultiPoint":
		rawPoints, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiPoint coordinates must be an array")
		}

		points := make([]geo.Point2D, 0, len(rawPoints))
		for _, raw := range rawPoints {
			point, err := parsePointCoordinate(raw)
			if err != nil {
				return nil, err
			}

			points = append(points, point)
		}

		return points, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (dummy-freefield supports Point/MultiPoint only)", feature.GeometryType)
	}
}

func lineStringsFromFeature(feature modelgeojson.Feature) ([][]geo.Point2D, error) {
	switch feature.GeometryType {
	case geometryTypeLineString:
		line, err := parseLineStringCoordinates(feature.Coordinates)
		if err != nil {
			return nil, err
		}

		return [][]geo.Point2D{line}, nil
	case geometryTypeMultiLineString:
		rawLines, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiLineString coordinates must be an array")
		}

		lines := make([][]geo.Point2D, 0, len(rawLines))
		for _, rawLine := range rawLines {
			line, err := parseLineStringCoordinates(rawLine)
			if err != nil {
				return nil, err
			}

			lines = append(lines, line)
		}

		return lines, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (cnossos-road supports LineString/MultiLineString only)", feature.GeometryType)
	}
}

func polygonsFromFeature(feature modelgeojson.Feature) ([][][]geo.Point2D, error) {
	switch feature.GeometryType {
	case "Polygon":
		polygon, err := parsePolygonCoordinates(feature.Coordinates)
		if err != nil {
			return nil, err
		}

		return [][][]geo.Point2D{polygon}, nil
	case "MultiPolygon":
		rawPolygons, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiPolygon coordinates must be an array")
		}

		polygons := make([][][]geo.Point2D, 0, len(rawPolygons))
		for _, rawPolygon := range rawPolygons {
			polygon, err := parsePolygonCoordinates(rawPolygon)
			if err != nil {
				return nil, err
			}

			polygons = append(polygons, polygon)
		}

		return polygons, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (cnossos-industry supports Point/MultiPoint/Polygon/MultiPolygon only)", feature.GeometryType)
	}
}

func flightTracksFromFeature(feature modelgeojson.Feature, options cnossosAircraftRunOptions) ([][]geo.Point3D, error) {
	switch feature.GeometryType {
	case geometryTypeLineString:
		line, err := parseFlightTrackCoordinates(feature.Coordinates, options)
		if err != nil {
			return nil, err
		}

		return [][]geo.Point3D{line}, nil
	case geometryTypeMultiLineString:
		rawLines, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiLineString coordinates must be an array")
		}

		lines := make([][]geo.Point3D, 0, len(rawLines))
		for _, rawLine := range rawLines {
			line, err := parseFlightTrackCoordinates(rawLine, options)
			if err != nil {
				return nil, err
			}

			lines = append(lines, line)
		}

		return lines, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (cnossos-aircraft supports LineString/MultiLineString only)", feature.GeometryType)
	}
}

func flightTracksFromFeatureBUF(feature modelgeojson.Feature, options bufAircraftRunOptions) ([][]geo.Point3D, error) {
	switch feature.GeometryType {
	case geometryTypeLineString:
		line, err := parseFlightTrackCoordinatesBUF(feature.Coordinates, options)
		if err != nil {
			return nil, err
		}

		return [][]geo.Point3D{line}, nil
	case geometryTypeMultiLineString:
		rawLines, ok := feature.Coordinates.([]any)
		if !ok {
			return nil, errors.New("geometry MultiLineString coordinates must be an array")
		}

		lines := make([][]geo.Point3D, 0, len(rawLines))
		for _, rawLine := range rawLines {
			line, err := parseFlightTrackCoordinatesBUF(rawLine, options)
			if err != nil {
				return nil, err
			}

			lines = append(lines, line)
		}

		return lines, nil
	default:
		return nil, fmt.Errorf("unsupported source geometry type %q (buf-aircraft supports LineString/MultiLineString only)", feature.GeometryType)
	}
}

//nolint:dupl // Aircraft track interpolation is kept separate because the option types differ.
func parseFlightTrackCoordinates(value any, options cnossosAircraftRunOptions) ([]geo.Point3D, error) {
	rawPoints, ok := value.([]any)
	if !ok {
		return nil, errors.New("line coordinates must be an array")
	}

	if len(rawPoints) < 2 {
		return nil, errors.New("line coordinates must contain at least 2 points")
	}

	points := make([]geo.Point3D, 0, len(rawPoints))

	lastIndex := len(rawPoints) - 1
	for i, rawPoint := range rawPoints {
		xy, z, hasZ, err := parsePointCoordinate3D(rawPoint)
		if err != nil {
			return nil, err
		}

		if !hasZ {
			fraction := 0.0
			if lastIndex > 0 {
				fraction = float64(i) / float64(lastIndex)
			}

			z = options.TrackStartHeightM + fraction*(options.TrackEndHeightM-options.TrackStartHeightM)
		}

		points = append(points, geo.Point3D{X: xy.X, Y: xy.Y, Z: z})
	}

	return points, nil
}

//nolint:dupl // Aircraft track interpolation is kept separate because the option types differ.
func parseFlightTrackCoordinatesBUF(value any, options bufAircraftRunOptions) ([]geo.Point3D, error) {
	rawPoints, ok := value.([]any)
	if !ok {
		return nil, errors.New("line coordinates must be an array")
	}

	if len(rawPoints) < 2 {
		return nil, errors.New("line coordinates must contain at least 2 points")
	}

	points := make([]geo.Point3D, 0, len(rawPoints))

	lastIndex := len(rawPoints) - 1
	for i, rawPoint := range rawPoints {
		xy, z, hasZ, err := parsePointCoordinate3D(rawPoint)
		if err != nil {
			return nil, err
		}

		if !hasZ {
			fraction := 0.0
			if lastIndex > 0 {
				fraction = float64(i) / float64(lastIndex)
			}

			z = options.TrackStartHeightM + fraction*(options.TrackEndHeightM-options.TrackStartHeightM)
		}

		points = append(points, geo.Point3D{X: xy.X, Y: xy.Y, Z: z})
	}

	return points, nil
}

func parsePointCoordinate3D(value any) (geo.Point2D, float64, bool, error) {
	raw, ok := value.([]any)
	if !ok {
		return geo.Point2D{}, 0, false, errors.New("point coordinates must be [x,y] or [x,y,z]")
	}

	if len(raw) < 2 {
		return geo.Point2D{}, 0, false, errors.New("point coordinates must have at least 2 values")
	}

	x, err := parseCoordinateNumber(raw[0])
	if err != nil {
		return geo.Point2D{}, 0, false, err
	}

	y, err := parseCoordinateNumber(raw[1])
	if err != nil {
		return geo.Point2D{}, 0, false, err
	}

	point := geo.Point2D{X: x, Y: y}
	if !point.IsFinite() {
		return geo.Point2D{}, 0, false, errors.New("point coordinates must be finite")
	}

	if len(raw) < 3 {
		return point, 0, false, nil
	}

	z, err := parseCoordinateNumber(raw[2])
	if err != nil {
		return geo.Point2D{}, 0, false, err
	}

	if math.IsNaN(z) || math.IsInf(z, 0) {
		return geo.Point2D{}, 0, false, errors.New("point z must be finite")
	}

	return point, z, true, nil
}

func parsePolygonCoordinates(value any) ([][]geo.Point2D, error) {
	rawRings, ok := value.([]any)
	if !ok || len(rawRings) == 0 {
		return nil, errors.New("polygon coordinates must contain at least one ring")
	}

	rings := make([][]geo.Point2D, 0, len(rawRings))
	for _, rawRing := range rawRings {
		ring, err := parseRingCoordinates(rawRing)
		if err != nil {
			return nil, err
		}

		rings = append(rings, ring)
	}

	return rings, nil
}

func parseRingCoordinates(value any) ([]geo.Point2D, error) {
	rawPoints, ok := value.([]any)
	if !ok || len(rawPoints) < 4 {
		return nil, errors.New("polygon ring must contain at least 4 points")
	}

	points := make([]geo.Point2D, 0, len(rawPoints))
	for _, rawPoint := range rawPoints {
		point, err := parsePointCoordinate(rawPoint)
		if err != nil {
			return nil, err
		}

		points = append(points, point)
	}

	return points, nil
}

func parseLineStringCoordinates(value any) ([]geo.Point2D, error) {
	rawPoints, ok := value.([]any)
	if !ok {
		return nil, errors.New("line coordinates must be an array")
	}

	if len(rawPoints) < 2 {
		return nil, errors.New("line coordinates must contain at least 2 points")
	}

	points := make([]geo.Point2D, 0, len(rawPoints))
	for _, rawPoint := range rawPoints {
		point, err := parsePointCoordinate(rawPoint)
		if err != nil {
			return nil, err
		}

		points = append(points, point)
	}

	return points, nil
}

func parseLineStringCoordinates3D(value any) ([]geo.Point2D, []float64, bool, error) {
	rawPoints, ok := value.([]any)
	if !ok {
		return nil, nil, false, errors.New("line coordinates must be an array")
	}

	if len(rawPoints) < 2 {
		return nil, nil, false, errors.New("line coordinates must contain at least 2 points")
	}

	points := make([]geo.Point2D, 0, len(rawPoints))
	elevations := make([]float64, 0, len(rawPoints))
	hasAnyZ := false
	hasMissingZ := false

	for _, rawPoint := range rawPoints {
		point, z, hasZ, err := parsePointCoordinate3D(rawPoint)
		if err != nil {
			return nil, nil, false, err
		}

		points = append(points, point)
		elevations = append(elevations, z)

		if hasZ {
			hasAnyZ = true
		} else {
			hasMissingZ = true
		}
	}

	if hasAnyZ && hasMissingZ {
		return nil, nil, false, errors.New("line coordinates must use either 2D points only or 3D points for every vertex")
	}

	if !hasAnyZ {
		return points, nil, false, nil
	}

	return points, elevations, true, nil
}

func parsePointCoordinate(value any) (geo.Point2D, error) {
	raw, ok := value.([]any)
	if !ok {
		return geo.Point2D{}, errors.New("point coordinates must be [x,y]")
	}

	if len(raw) < 2 {
		return geo.Point2D{}, errors.New("point coordinates must have at least 2 values")
	}

	x, err := parseCoordinateNumber(raw[0])
	if err != nil {
		return geo.Point2D{}, err
	}

	y, err := parseCoordinateNumber(raw[1])
	if err != nil {
		return geo.Point2D{}, err
	}

	point := geo.Point2D{X: x, Y: y}
	if !point.IsFinite() {
		return geo.Point2D{}, errors.New("point coordinates must be finite")
	}

	return point, nil
}

func parseCoordinateNumber(value any) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid numeric coordinate %q: %w", typed, err)
		}

		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported coordinate type %T", value)
	}
}
