package lglnimport

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	"github.com/aconiq/backend/internal/io/citygmlimport"
)

// ImportFormat is the import_format property every feature Load returns
// carries, so a consumer can tell LGLN buildings from OSM or drawn ones.
const ImportFormat = "lgln-lod2"

// defaultTileEPSG is the CRS LGLN publishes its tiles in (ETRS89 / UTM 32N).
// It is the fallback for a tile whose srsName the parser does not recognise.
const defaultTileEPSG = 25832

// Result is what Load hands back: the buildings in WGS84, the tiles they came
// from, and how many buildings the parser left out, by reason.
type Result struct {
	Collection modelgeojson.FeatureCollection
	Tiles      []Tile
	Skipped    map[string]int
}

// Load finds the tiles intersecting bb, fetches them into cacheDir, parses
// them and returns one building feature per CityGML Building or BuildingPart
// whose footprint centroid lies inside bb, in EPSG:4326.
//
// The centroid rule decides membership so that a building straddling the box
// edge is either in or out, never both: the frontend removes the OSM buildings
// it replaces by the same rule. A building listed in two tiles is kept once,
// from the first tile in Search order.
func (c *Client) Load(ctx context.Context, bb BBox, cacheDir string) (Result, error) {
	tiles, err := c.Search(ctx, bb)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Collection: modelgeojson.FeatureCollection{
			Type:     modelgeojson.TypeFeatureCollection,
			Features: []modelgeojson.GeoJSONFeature{},
		},
		Tiles:   tiles,
		Skipped: map[string]int{},
	}

	seen := make(map[string]struct{})

	for _, tile := range tiles {
		path, _, fetchErr := c.FetchTile(ctx, tile, cacheDir)
		if fetchErr != nil {
			return Result{}, fetchErr
		}

		features, skipped, tileErr := tileBuildings(path, tile, bb)
		if tileErr != nil {
			return Result{}, tileErr
		}

		for reason, n := range skipped {
			result.Skipped[reason] += n
		}

		for _, feature := range features {
			id, _ := feature.Properties["id"].(string)
			if _, dup := seen[id]; dup {
				continue
			}

			seen[id] = struct{}{}

			result.Collection.Features = append(result.Collection.Features, feature)
		}
	}

	return result, nil
}

// tileBuildings parses one cached tile and returns its buildings inside bb,
// reprojected to WGS84, plus the parser's skip counts.
func tileBuildings(path string, tile Tile, bb BBox) ([]modelgeojson.GeoJSONFeature, map[string]int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read tile %s: %w", tile.ID, err)
	}

	parsed, err := citygmlimport.ReadWithCRS(data)

	skipped := make(map[string]int)
	for _, s := range parsed.Report.Details {
		skipped[string(s.Reason)]++
	}

	if err != nil {
		// A tile of fields and forest has no building at all, and one whose
		// every building was skipped has none usable. Both are empty tiles,
		// not broken ones; the second still reports what it skipped.
		if errors.Is(err, citygmlimport.ErrNoBuildings) {
			return nil, skipped, nil
		}

		return nil, nil, fmt.Errorf("parse tile %s: %w", tile.ID, err)
	}

	code := parsed.EPSGCode
	if code == 0 {
		code = defaultTileEPSG
	}

	pipeline, err := wgs84Pipeline(code)
	if err != nil {
		return nil, nil, fmt.Errorf("tile %s: CRS transform: %w", tile.ID, err)
	}

	out := make([]modelgeojson.GeoJSONFeature, 0, len(parsed.Collection.Features))

	for _, feature := range parsed.Collection.Features {
		rings, ok := polygonRings(feature.Geometry.Coordinates)
		if !ok {
			continue
		}

		centroid, ok := geo.PolygonCentroid(rings)
		if !ok {
			continue
		}

		lonLat, err := pipeline.ApplyPoint(centroid)
		if err != nil {
			return nil, nil, fmt.Errorf("tile %s: CRS transform: %w", tile.ID, err)
		}

		if !bb.Contains(lonLat) {
			continue
		}

		coords, err := transformRings(rings, pipeline)
		if err != nil {
			return nil, nil, fmt.Errorf("tile %s: CRS transform: %w", tile.ID, err)
		}

		feature.Properties["import_format"] = ImportFormat
		feature.Properties["lgln_tile"] = tile.ID
		feature.Geometry.Coordinates = coords
		out = append(out, feature)
	}

	return out, skipped, nil
}

// Contains reports whether a WGS84 point lies in the box, edges included. The
// frontend and `aconiq import --from-lgln` decide which OSM buildings an LGLN
// import replaces by the same test, so they must agree exactly, edges included.
func (b BBox) Contains(p geo.Point2D) bool {
	return p.X >= b.West && p.X <= b.East && p.Y >= b.South && p.Y <= b.North
}

// polygonRings reads the []any polygon coordinates citygmlimport emits.
func polygonRings(coords any) ([][]geo.Point2D, bool) {
	rawRings, ok := coords.([]any)
	if !ok || len(rawRings) == 0 {
		return nil, false
	}

	rings := make([][]geo.Point2D, 0, len(rawRings))

	for _, rawRing := range rawRings {
		rawPoints, ok := rawRing.([]any)
		if !ok {
			return nil, false
		}

		ring := make([]geo.Point2D, 0, len(rawPoints))

		for _, rawPoint := range rawPoints {
			pair, ok := rawPoint.([]any)
			if !ok || len(pair) < 2 {
				return nil, false
			}

			x, okX := pair[0].(float64)
			y, okY := pair[1].(float64)

			if !okX || !okY {
				return nil, false
			}

			ring = append(ring, geo.Point2D{X: x, Y: y})
		}

		rings = append(rings, ring)
	}

	return rings, true
}

func transformRings(rings [][]geo.Point2D, pipeline geo.TransformPipeline) ([]any, error) {
	out := make([]any, 0, len(rings))

	for _, ring := range rings {
		points := make([]any, 0, len(ring))

		for _, p := range ring {
			q, err := pipeline.ApplyPoint(p)
			if err != nil {
				return nil, fmt.Errorf("transform %v: %w", p, err)
			}

			points = append(points, []any{q.X, q.Y})
		}

		out = append(out, points)
	}

	return out, nil
}

// wgs84Pipeline builds the transform from a tile's EPSG code to EPSG:4326.
func wgs84Pipeline(code int) (geo.TransformPipeline, error) {
	from, err := geo.ParseCRS("EPSG:" + strconv.Itoa(code))
	if err != nil {
		return geo.TransformPipeline{}, fmt.Errorf("tile CRS: %w", err)
	}

	to, err := geo.ParseCRS("EPSG:4326")
	if err != nil {
		return geo.TransformPipeline{}, fmt.Errorf("target CRS: %w", err)
	}

	pipeline, err := geo.BuildTransformPipeline(to, from)
	if err != nil {
		return geo.TransformPipeline{}, fmt.Errorf("build transform: %w", err)
	}

	return pipeline, nil
}
