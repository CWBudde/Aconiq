// Package citygmlimport reads a narrow CityGML building subset and converts it
// into the project model format.
package citygmlimport

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

type polygon3D struct {
	ring []coord3D
}

type coord3D struct {
	X float64
	Y float64
	Z float64
}

type buildingCandidate struct {
	ID             string
	MeasuredHeight *float64
	Polygons       []polygon3D
}

// Read reads a narrow CityGML building subset and returns a GeoJSON-compatible
// FeatureCollection ready for Normalize.
func Read(data []byte) (modelgeojson.FeatureCollection, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))

	features := make([]modelgeojson.GeoJSONFeature, 0)
	buildingIndex := 0

	for {
		tok, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return modelgeojson.FeatureCollection{}, fmt.Errorf("citygml: decode xml: %w", err)
		}

		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		if start.Name.Local != "Building" {
			continue
		}

		building, err := decodeBuilding(decoder, start)
		if err != nil {
			return modelgeojson.FeatureCollection{}, err
		}

		feature, ok, err := buildingToFeature(building, buildingIndex)
		if err != nil {
			return modelgeojson.FeatureCollection{}, err
		}

		if ok {
			features = append(features, feature)
			buildingIndex++
		}
	}

	if len(features) == 0 {
		return modelgeojson.FeatureCollection{}, errors.New("citygml: no supported building features found")
	}

	return modelgeojson.FeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
	}, nil
}

func decodeBuilding(decoder *xml.Decoder, start xml.StartElement) (buildingCandidate, error) {
	building := buildingCandidate{
		ID: attrValue(start.Attr, "id"),
	}

	for {
		tok, err := decoder.Token()
		if err != nil {
			return buildingCandidate{}, fmt.Errorf("citygml: read building: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "measuredHeight":
				text, err := readElementText(decoder)
				if err != nil {
					return buildingCandidate{}, err
				}

				value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
				if err != nil {
					return buildingCandidate{}, fmt.Errorf("citygml: parse measuredHeight for building %q: %w", building.ID, err)
				}

				building.MeasuredHeight = &value
			case "posList":
				text, err := readElementText(decoder)
				if err != nil {
					return buildingCandidate{}, err
				}

				ring, err := parsePosList(text)
				if err != nil {
					return buildingCandidate{}, fmt.Errorf("citygml: parse posList for building %q: %w", building.ID, err)
				}

				building.Polygons = append(building.Polygons, polygon3D{ring: ring})
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return building, nil
			}
		}
	}
}

func readElementText(decoder *xml.Decoder) (string, error) {
	var text strings.Builder

	for {
		tok, err := decoder.Token()
		if err != nil {
			return "", fmt.Errorf("citygml: read element text: %w", err)
		}

		switch t := tok.(type) {
		case xml.CharData:
			text.Write([]byte(t))
		case xml.EndElement:
			return text.String(), nil
		}
	}
}

func parsePosList(raw string) ([]coord3D, error) {
	fields := strings.Fields(raw)
	if len(fields) < 12 {
		return nil, errors.New("posList must contain at least four 3D coordinates")
	}

	if len(fields)%3 != 0 {
		return nil, errors.New("posList must contain coordinates in 3D triplets")
	}

	ring := make([]coord3D, 0, len(fields)/3)
	for i := 0; i < len(fields); i += 3 {
		x, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return nil, err
		}

		y, err := strconv.ParseFloat(fields[i+1], 64)
		if err != nil {
			return nil, err
		}

		z, err := strconv.ParseFloat(fields[i+2], 64)
		if err != nil {
			return nil, err
		}

		ring = append(ring, coord3D{X: x, Y: y, Z: z})
	}

	if !sameXY(ring[0], ring[len(ring)-1]) {
		ring = append(ring, ring[0])
	}

	return ring, nil
}

func buildingToFeature(building buildingCandidate, index int) (modelgeojson.GeoJSONFeature, bool, error) {
	if len(building.Polygons) == 0 {
		return modelgeojson.GeoJSONFeature{}, false, nil
	}

	footprint, ok := selectFootprint(building.Polygons)
	if !ok {
		return modelgeojson.GeoJSONFeature{}, false, nil
	}

	height := measuredOrComputedHeight(building)
	if !(height > 0) || math.IsNaN(height) || math.IsInf(height, 0) {
		return modelgeojson.GeoJSONFeature{}, false, nil
	}

	id := strings.TrimSpace(building.ID)
	if id == "" {
		id = fmt.Sprintf("citygml-building-%03d", index)
	}

	coords := make([]any, 0, len(footprint))
	for _, pt := range footprint {
		coords = append(coords, []any{pt.X, pt.Y})
	}

	return modelgeojson.GeoJSONFeature{
		Type: "Feature",
		Properties: map[string]any{
			"id":                id,
			"kind":              "building",
			"height_m":          height,
			"import_format":     "citygml",
			"citygml_source_id": id,
		},
		Geometry: modelgeojson.Geometry{
			Type:        "Polygon",
			Coordinates: []any{coords},
		},
	}, true, nil
}

func selectFootprint(polygons []polygon3D) ([]coord3D, bool) {
	bestIndex := -1
	bestZ := math.MaxFloat64

	for i, polygon := range polygons {
		if len(polygon.ring) < 4 {
			continue
		}

		sum := 0.0
		for _, pt := range polygon.ring {
			sum += pt.Z
		}

		avgZ := sum / float64(len(polygon.ring))
		if avgZ < bestZ {
			bestZ = avgZ
			bestIndex = i
		}
	}

	if bestIndex < 0 {
		return nil, false
	}

	return polygons[bestIndex].ring, true
}

func measuredOrComputedHeight(building buildingCandidate) float64 {
	if building.MeasuredHeight != nil {
		return *building.MeasuredHeight
	}

	minZ := math.MaxFloat64
	maxZ := -math.MaxFloat64

	for _, polygon := range building.Polygons {
		for _, pt := range polygon.ring {
			if pt.Z < minZ {
				minZ = pt.Z
			}

			if pt.Z > maxZ {
				maxZ = pt.Z
			}
		}
	}

	if minZ == math.MaxFloat64 || maxZ == -math.MaxFloat64 {
		return math.NaN()
	}

	return maxZ - minZ
}

func sameXY(a, b coord3D) bool {
	return a.X == b.X && a.Y == b.Y
}

func attrValue(attrs []xml.Attr, local string) string {
	for _, attr := range attrs {
		if attr.Name.Local == local {
			return strings.TrimSpace(attr.Value)
		}
	}

	return ""
}
