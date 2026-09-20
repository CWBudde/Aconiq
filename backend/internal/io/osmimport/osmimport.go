// Package osmimport fetches OpenStreetMap data via the Overpass API and
// converts it into the project model's GeoJSON FeatureCollection format.
package osmimport

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/aconiq/backend/internal/buildinfo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
	overpass "github.com/cwbudde/go-overpass"
)

const (
	// publicOverpassURL is the default Overpass API server used when no custom server is configured.
	publicOverpassURL  = "https://overpass-api.de/api/interpreter" //nolint:gosec // not a credential
	defaultTimeoutSecs = 25
	// defaultBarrierHeightM is the assumed height for walls/fences when no height tag is present.
	defaultBarrierHeightM = 2.0
	// defaultBuildingHeightM is the height assumed for a building whose way carries
	// neither `height` nor `building:levels` — three storeys at metersPerLevel.
	//
	// It is an assumption, not a measurement, so every feature that receives it
	// also carries propertyHeightSource. See buildingHeight.
	defaultBuildingHeightM = 3 * metersPerLevel
	metersPerLevel         = 3.0
	minPolygonPoints       = 4

	// propertyHeightSource records that this feature's height_m was assumed
	// rather than read from a tag. It is written only in the assumed case:
	// its absence records no provenance, not "read from OSM".
	propertyHeightSource = "height_source"
	// heightSourceAssumed is propertyHeightSource's only value today.
	heightSourceAssumed = "assumed"

	// contactURL is the project the Overpass usage policy asks a caller to be
	// reachable at. It is part of userAgent and nothing else.
	contactURL = "https://github.com/CWBudde/Aconiq"
)

// userAgent is what every Overpass request identifies itself as.
//
// The Overpass usage policy asks for a named agent with a way to make contact,
// and overpass-api.de enforces it: a request carrying Go's default
// `Go-http-client/1.1` is answered with 406, while the same query with a name
// returns 200. go-overpass sets no User-Agent of its own (it sets Content-Type
// and nothing else), so without this every import is one server-side policy
// change away from failing, and did fail from at least one network.
func userAgent() string {
	return buildinfo.Name + "/" + buildinfo.Version() + " (+" + contactURL + ")"
}

// userAgentClient names the caller on every request and delegates the rest.
//
// go-overpass exposes no header hook — its only seam is the HTTPClient
// interface it takes — so this wraps whatever client is in play rather than
// configuring one. Config.HTTPClient is set by tests alone; neither the HTTP
// handler nor the CLI passes one, so the wrapping has to happen inside Fetch
// and not at the call sites.
type userAgentClient struct {
	inner overpass.HTTPClient
}

func (c userAgentClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", userAgent())

	resp, err := c.inner.Do(req)
	if err != nil {
		return nil, fmt.Errorf("overpass request: %w", err)
	}

	return resp, nil
}

// BBox is a geographic bounding box in WGS84 degrees.
type BBox struct {
	South, West, North, East float64
}

// Config configures an OSM import operation.
type Config struct {
	BBox             BBox
	OverpassEndpoint string              // optional; default: https://overpass-api.de/api/interpreter
	QueryTimeoutSecs int                 // default: 25
	HTTPClient       overpass.HTTPClient // optional; nil = use default http.DefaultClient
}

// Fetch queries the Overpass API for the configured bounding box and returns
// a GeoJSON FeatureCollection ready for modelgeojson.Normalize.
func Fetch(ctx context.Context, cfg Config) (modelgeojson.FeatureCollection, error) {
	endpoint := cfg.OverpassEndpoint
	if endpoint == "" {
		endpoint = publicOverpassURL
	}

	timeoutSecs := cfg.QueryTimeoutSecs
	if timeoutSecs <= 0 {
		timeoutSecs = defaultTimeoutSecs
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	client := overpass.NewWithSettings(endpoint, 1, userAgentClient{inner: httpClient})
	query := buildQuery(cfg.BBox, timeoutSecs)

	result, err := client.QueryContext(ctx, query)
	if err != nil {
		return modelgeojson.FeatureCollection{}, fmt.Errorf("overpass query: %w", err)
	}

	features := make([]modelgeojson.GeoJSONFeature, 0, len(result.Ways))

	for _, way := range result.Ways {
		feat, ok := wayToFeature(way)
		if !ok {
			continue
		}

		features = append(features, feat)
	}

	return modelgeojson.FeatureCollection{
		Type:     modelgeojson.TypeFeatureCollection,
		Features: features,
	}, nil
}

// buildQuery builds the raw Overpass QL compound query for the given bbox and timeout.
func buildQuery(bbox BBox, timeoutSecs int) string {
	b := fmt.Sprintf("%g,%g,%g,%g", bbox.South, bbox.West, bbox.North, bbox.East)

	return fmt.Sprintf(
		"[out:json][timeout:%d];\n(\n  way[\"highway\"](%s);\n  way[\"railway\"~\"^(rail|tram)$\"](%s);\n  way[\"building\"](%s);\n  way[\"barrier\"~\"^(wall|fence)$\"](%s);\n);\nout geom;\n",
		timeoutSecs, b, b, b, b,
	)
}

// wayToFeature converts a Way to a GeoJSON feature.
// Returns false to skip the way if it has fewer than 2 geometry points
// or if no recognised OSM tag is found.
func wayToFeature(way *overpass.Way) (modelgeojson.GeoJSONFeature, bool) {
	if len(way.Geometry) < 2 {
		return modelgeojson.GeoJSONFeature{}, false
	}

	tags := way.Tags
	if tags == nil {
		tags = map[string]string{}
	}

	props := map[string]any{
		"osm_id": strconv.FormatInt(way.ID, 10),
	}

	geomType, coords, ok := classifyWay(way, tags, props)
	if !ok {
		return modelgeojson.GeoJSONFeature{}, false
	}

	return modelgeojson.GeoJSONFeature{
		Type:       "Feature",
		ID:         "osm-way-" + strconv.FormatInt(way.ID, 10),
		Properties: props,
		Geometry: modelgeojson.Geometry{
			Type:        geomType,
			Coordinates: coords,
		},
	}, true
}

// classifyWay fills props based on OSM tags and returns the geometry type and coordinates.
// Returns ok=false if the way should be skipped.
func classifyWay(way *overpass.Way, tags map[string]string, props map[string]any) (geomType string, coords any, ok bool) {
	switch {
	case tags["highway"] != "":
		applyHighwayProps(tags, props)

		return modelgeojson.GeometryTypeLineString, lineCoords(way.Geometry), true

	case tags["railway"] != "":
		applyRailwayProps(tags, props)

		return modelgeojson.GeometryTypeLineString, lineCoords(way.Geometry), true

	case tags["building"] != "":
		ring := polygonCoords(way.Geometry)
		if ring == nil {
			return "", nil, false
		}

		applyBuildingProps(tags, props)

		return modelgeojson.GeometryTypePolygon, ring, true

	case tags["barrier"] != "":
		applyBarrierProps(tags, props)

		return modelgeojson.GeometryTypeLineString, lineCoords(way.Geometry), true

	default:
		return "", nil, false
	}
}

// applyHighwayProps sets props for a highway (road source) way.
func applyHighwayProps(tags map[string]string, props map[string]any) {
	props["kind"] = "source"
	props["source_type"] = "line"
	props["highway"] = tags["highway"]

	if v, ok := parseTagFloat(tags["maxspeed"]); ok {
		props["maxspeed_kmh"] = v
	}

	if v, ok := parseTagFloat(tags["lanes"]); ok {
		props["lanes"] = v
	}

	if s := tags["surface"]; s != "" {
		props["surface"] = s
	}
}

// applyRailwayProps sets props for a railway (rail source) way.
func applyRailwayProps(tags map[string]string, props map[string]any) {
	props["kind"] = "source"
	props["source_type"] = "line"
	props["railway"] = tags["railway"]

	if v, ok := parseTagFloat(tags["maxspeed"]); ok {
		props["maxspeed_kmh"] = v
	}
}

// applyBuildingProps sets props for a building way.
func applyBuildingProps(tags map[string]string, props map[string]any) {
	props["kind"] = "building"
	props["building"] = tags["building"]

	if v := tags["building:levels"]; v != "" {
		props["building:levels"] = v
	}

	setHeight(props, buildingHeight(tags))
}

// applyBarrierProps sets props for a barrier way.
func applyBarrierProps(tags map[string]string, props map[string]any) {
	props["kind"] = "barrier"
	props["barrier"] = tags["barrier"]

	setHeight(props, barrierHeight(tags))
}

// height is a height_m together with where it came from.
type height struct {
	meters  float64
	assumed bool
}

// setHeight writes the height and, when it was assumed, says so on the feature.
func setHeight(props map[string]any, h height) {
	props["height_m"] = h.meters

	if h.assumed {
		props[propertyHeightSource] = heightSourceAssumed
	}
}

// buildingHeight tries the height tag (stripping "m" suffix), then
// building:levels × 3.0, then defaultBuildingHeightM.
//
// The last step is the one worth defending. It used to return nothing, which
// looks like the careful answer and is not: `height_m` is required on a
// building feature, so every untagged building made the whole import
// unsaveable — 172 of 580 in a Berlin extract, which is an ordinary proportion
// for a German one. Meanwhile the map was drawing those buildings anyway,
// because the frontend normalizer quietly assumed 9 m of its own. So the
// choice was never "assume or do not"; it was "assume once, here, on the
// record" against "assume twice, in two places, silently". A caller that must
// not compute over an assumed height reads propertyHeightSource.
func buildingHeight(tags map[string]string) height {
	if h, ok := parseHeightTag(tags["height"]); ok {
		return height{meters: h}
	}

	if levels, ok := parseTagFloat(tags["building:levels"]); ok {
		return height{meters: levels * metersPerLevel}
	}

	return height{meters: defaultBuildingHeightM, assumed: true}
}

// barrierHeight tries the height tag; if not found returns 2.0 m, which is a
// reasonable default for walls and fences — marked assumed for the same reason
// a building's default is.
func barrierHeight(tags map[string]string) height {
	if h, ok := parseHeightTag(tags["height"]); ok {
		return height{meters: h}
	}

	return height{meters: defaultBarrierHeightM, assumed: true}
}

// parseHeightTag strips " m" and "m" suffix, then parses as float64.
func parseHeightTag(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}

	s = strings.TrimSuffix(s, " m")
	s = strings.TrimSuffix(s, "m")
	s = strings.TrimSpace(s)

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}

	return v, true
}

// parseTagFloat parses a tag value as float64, returning false if empty or unparseable.
func parseTagFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}

	return v, true
}

// lineCoords converts []Point{Lat, Lon} to []any{[]any{lon, lat}, ...}.
// GeoJSON coordinates are [longitude, latitude].
func lineCoords(geom []overpass.Point) []any {
	coords := make([]any, len(geom))
	for i, p := range geom {
		coords[i] = []any{p.Lon, p.Lat}
	}

	return coords
}

// polygonCoords converts geometry points to a GeoJSON polygon coordinate ring
// ([]any{ring}) where ring is []any{[]any{lon, lat}, ...}.
// Ensures the ring is closed (first point == last point).
// Returns nil if fewer than 4 points after closing.
func polygonCoords(geom []overpass.Point) []any {
	if len(geom) < 2 {
		return nil
	}

	// Build the ring with capacity for a possible closure point.
	ring := make([]any, 0, len(geom)+1)
	for _, p := range geom {
		ring = append(ring, []any{p.Lon, p.Lat})
	}

	// Ensure ring is closed: append first point if not equal to last.
	firstPt, firstOK := ring[0].([]any)
	lastPt, lastOK := ring[len(ring)-1].([]any)

	if firstOK && lastOK && (firstPt[0] != lastPt[0] || firstPt[1] != lastPt[1]) {
		ring = append(ring, []any{firstPt[0], firstPt[1]})
	}

	if len(ring) < minPolygonPoints {
		return nil
	}

	return []any{ring}
}
