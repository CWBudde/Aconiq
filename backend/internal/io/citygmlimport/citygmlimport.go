// Package citygmlimport reads CityGML files and converts building data
// into the project model format using the go-citygml library.
package citygmlimport

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/cwbudde/go-citygml/citygml"
	"github.com/cwbudde/go-citygml/helpers"
	"github.com/cwbudde/go-citygml/types"
)

// maxXMLNestingDepth bounds element nesting in an untrusted CityGML document.
//
// Go's xml.Decoder is safe against XXE and entity expansion, but it keeps one
// stack entry per open element and imposes no depth limit, so a document that
// is nothing but opening tags turns a small upload into a large allocation and
// a deep walk through the scanner. Real CityGML nests around a dozen levels
// (CityModel > cityObjectMember > Building > boundedBy > WallSurface >
// lodNMultiSurface > MultiSurface > surfaceMember > Polygon > exterior >
// LinearRing > posList); 256 leaves an order of magnitude of headroom.
const maxXMLNestingDepth = 256

// checkXMLNestingDepth walks the document's tokens and rejects it when element
// nesting exceeds maxXMLNestingDepth. It runs before the CityGML parser so the
// deep document is never handed to a decoder that would keep it in memory.
// Malformed XML is not reported here; the parser produces the better message.
func checkXMLNestingDepth(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))

	depth := 0

	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return nil
		}

		switch tok.(type) {
		case xml.StartElement:
			depth++
			if depth > maxXMLNestingDepth {
				return fmt.Errorf("citygml: element nesting deeper than %d levels", maxXMLNestingDepth)
			}
		case xml.EndElement:
			depth--
		}
	}
}

// ErrNoBuildings reports a document that yielded no importable building.
var ErrNoBuildings = errors.New("citygml: no supported building features found")

// SkipReason describes why a building was excluded from the import.
type SkipReason string

const (
	SkipNoHeight       SkipReason = "no height available"
	SkipInvalidHeight  SkipReason = "height is zero, NaN, or Inf"
	SkipNoFootprint    SkipReason = "no footprint geometry"
	SkipDegeneratePoly SkipReason = "footprint has fewer than 4 points"
	SkipSelfIntersects SkipReason = "footprint crosses itself"
)

// SkippedBuilding records a building that was excluded during import.
type SkippedBuilding struct {
	ID     string     `json:"id"`
	Reason SkipReason `json:"reason"`
}

// ImportReport summarises the outcome of a CityGML import.
type ImportReport struct {
	Total    int               `json:"total"`
	Imported int               `json:"imported"`
	Skipped  int               `json:"skipped"`
	Details  []SkippedBuilding `json:"skipped_buildings,omitempty"`
}

// ReadResult holds the result of reading a CityGML document.
type ReadResult struct {
	Collection modelgeojson.FeatureCollection
	EPSGCode   int // 0 if CRS could not be determined
	Report     ImportReport
}

// Read reads a CityGML document and returns a GeoJSON-compatible
// FeatureCollection ready for Normalize.
func Read(data []byte) (modelgeojson.FeatureCollection, error) {
	result, err := ReadWithCRS(data)
	if err != nil {
		return modelgeojson.FeatureCollection{}, err
	}

	return result.Collection, nil
}

// ReadWithCRS reads a CityGML document and also extracts the CRS from the document's srsName.
func ReadWithCRS(data []byte) (ReadResult, error) {
	err := checkXMLNestingDepth(data)
	if err != nil {
		return ReadResult{}, err
	}

	doc, err := citygml.Read(bytes.NewReader(data), citygml.Options{})
	if err != nil {
		return ReadResult{}, fmt.Errorf("citygml: %w", err)
	}

	candidates := importCandidates(doc.Buildings)
	features := make([]modelgeojson.GeoJSONFeature, 0, len(candidates))
	report := ImportReport{Total: len(candidates)}

	for i, c := range candidates {
		feature, reason := buildingToFeature(c.building, i)
		if reason == "" {
			if c.parentID != "" {
				feature.Properties["citygml_parent_id"] = c.parentID
			}

			features = append(features, feature)
		} else {
			id := strings.TrimSpace(c.building.ID)
			if id == "" {
				id = fmt.Sprintf("citygml-building-%03d", i)
			}

			report.Details = append(report.Details, SkippedBuilding{
				ID:     id,
				Reason: reason,
			})
		}
	}

	report.Imported = len(features)
	report.Skipped = report.Total - report.Imported

	if len(features) == 0 {
		return ReadResult{Report: report}, ErrNoBuildings
	}

	return ReadResult{
		Collection: modelgeojson.FeatureCollection{
			Type:     modelgeojson.TypeFeatureCollection,
			Features: features,
		},
		EPSGCode: doc.CRS.Code,
		Report:   report,
	}, nil
}

// candidate is one Building or BuildingPart that may become a feature.
type candidate struct {
	building *types.Building
	parentID string // the enclosing Building's gml:id; empty for a Building
}

// importCandidates flattens buildings and their parts, depth-first in
// document order. Every part becomes its own feature, with its own height and
// footprint: LoD2 data splits a building into parts exactly where the roof
// height changes, which is what a noise model needs to see. A building made
// only of parts (no footprint of its own) is a container and is left out
// rather than reported as skipped.
func importCandidates(buildings []types.Building) []candidate {
	var out []candidate

	var walk func(b *types.Building, parentID string)

	walk = func(b *types.Building, parentID string) {
		if len(b.Parts) == 0 || b.Footprint != nil {
			out = append(out, candidate{building: b, parentID: parentID})
		}

		for i := range b.Parts {
			walk(&b.Parts[i], strings.TrimSpace(b.ID))
		}
	}

	for i := range buildings {
		walk(&buildings[i], "")
	}

	return out
}

// buildingToFeature converts a CityGML building to a GeoJSON feature.
// Returns the feature and an empty SkipReason on success, or a zero feature
// and the reason on failure.
func buildingToFeature(b *types.Building, index int) (modelgeojson.GeoJSONFeature, SkipReason) {
	// Get effective height.
	height, hasHeight := helpers.BuildingHeight(b)
	if !hasHeight {
		return modelgeojson.GeoJSONFeature{}, SkipNoHeight
	}

	if !(height > 0) || math.IsNaN(height) || math.IsInf(height, 0) {
		return modelgeojson.GeoJSONFeature{}, SkipInvalidHeight
	}

	// Get footprint polygon.
	if b.Footprint == nil {
		return modelgeojson.GeoJSONFeature{}, SkipNoFootprint
	}

	if len(b.Footprint.Exterior.Points) < 4 {
		return modelgeojson.GeoJSONFeature{}, SkipDegeneratePoly
	}

	// A footprint that crosses itself has no reliable inside, and validation
	// refuses the whole model over it. Surveyed data does contain the odd one
	// (a ring doubling back over a shared wall), so drop that building and
	// say so rather than lose every other one in the file.
	if modelgeojson.RingSelfIntersects(ringPairs(b.Footprint.Exterior)) {
		return modelgeojson.GeoJSONFeature{}, SkipSelfIntersects
	}

	// Build ID.
	id := strings.TrimSpace(b.ID)
	if id == "" {
		id = fmt.Sprintf("citygml-building-%03d", index)
	}

	// Convert footprint to GeoJSON coordinates: the exterior ring, then any
	// courtyard holes that are rings at all.
	rings := []any{ringCoordinates(b.Footprint.Exterior)}
	for _, hole := range b.Footprint.Interior {
		if len(hole.Points) >= 4 {
			rings = append(rings, ringCoordinates(hole))
		}
	}

	props := map[string]any{
		"id":                id,
		"kind":              "building",
		"height_m":          height,
		"import_format":     "citygml",
		"citygml_source_id": id,
	}

	if b.Class != "" {
		props["citygml_class"] = b.Class
	}

	if b.Function != "" {
		props["citygml_function"] = b.Function
	}

	if b.Usage != "" {
		props["citygml_usage"] = b.Usage
	}

	if b.LoD != "" {
		props["citygml_lod"] = string(b.LoD)
	}

	return modelgeojson.GeoJSONFeature{
		Type:       "Feature",
		Properties: props,
		Geometry: modelgeojson.Geometry{
			Type:        modelgeojson.GeometryTypePolygon,
			Coordinates: rings,
		},
	}, ""
}

// ringCoordinates drops Z: the model's footprints are 2D.
func ringCoordinates(ring types.Ring) []any {
	coords := make([]any, 0, len(ring.Points))
	for _, pt := range ring.Points {
		coords = append(coords, []any{pt.X, pt.Y})
	}

	return coords
}

func ringPairs(ring types.Ring) [][2]float64 {
	pairs := make([][2]float64, len(ring.Points))
	for i, pt := range ring.Points {
		pairs[i] = [2]float64{pt.X, pt.Y}
	}

	return pairs
}
