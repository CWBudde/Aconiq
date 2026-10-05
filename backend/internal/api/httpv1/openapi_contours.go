package httpv1

import (
	"github.com/aconiq/backend/internal/report/contour"
)

// openapiRunContourPathItems describes the contour endpoint. Go 1.22 routing
// gives it precedence over /api/v1/runs/{id}, the same way the log endpoint has
// it.
func openapiRunContourPathItems() map[string]any {
	return map[string]any{
		"/api/v1/runs/{id}/contours": map[string]any{
			"get": map[string]any{
				"summary":     "Contour lines for a run's result raster",
				"operationId": "getRunContours",
				"description": "Generates ISO-band contours from the run's result raster, in the CRS asked " +
					"for. Every band the raster carries is returned in one response, told apart by " +
					"`band_name`; a client showing one band filters rather than asking again. The response " +
					"is a JSON envelope and not a GeoJSON FeatureCollection because RFC 7946 fixes " +
					"GeoJSON to WGS84 and so cannot declare the CRS these coordinates are actually in. " +
					"Only a run computed over a grid has a raster: a run over individually placed " +
					"receivers is refused with `" + errorCodeRunHasNoRaster + "`.",
				"parameters": []map[string]any{
					{
						"name":        "id",
						"in":          "path",
						"required":    true,
						"description": "Run ID",
						"schema":      map[string]any{"type": "string"},
					},
					{
						"name":        "interval",
						"in":          "query",
						"required":    false,
						"description": "dB step between contour levels. Must be positive; omitted it defaults to the EU END convention.",
						"schema":      map[string]any{"type": "number", "default": contour.DefaultInterval},
					},
					{
						"name":     "crs",
						"in":       "query",
						"required": false,
						"description": "CRS to return the contours in, as an EPSG identifier. Defaults to `" +
							defaultContourCRS + "`. The response always names the CRS it is actually in.",
						"schema": map[string]any{"type": "string", "default": defaultContourCRS},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Contour lines, with the CRS and the dB interval they were generated at",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/ContourResult",
								},
							},
						},
					},
					"400": openapiErrorResponse(
						"Missing run ID, a non-positive or unparseable `interval`, or a `crs` that is not a " +
							"recognised identifier (`" + errorCodeBadRequest + "`); or a CRS at either end " +
							"carries no EPSG code, so the contours cannot be moved into it (`" +
							errorCodeCRSNotProjectable + "`, the same code POST /api/v1/transform answers with " +
							"for the same cause). A different `crs` on the same run answers.",
					),
					"404": openapiErrorResponse(
						"Project not initialized, or no run with this ID (`" + errorCodeNotFound +
							"`); or the run exists but recorded no result raster (`" + errorCodeRunHasNoRaster +
							"`), which is what a receiver mode other than an auto grid produces.",
					),
					"405": methodNotAllowedResponse(),
					"409": openapiErrorResponse(
						"The run's raster declares no georeference (`" + errorCodeRunHasNoGrid + "`): it was " +
							"computed over receivers placed individually, which record no cell size, so no " +
							"`interval` and no `crs` would make this call succeed. The message is the contour " +
							"package's own, worded as `aconiq export` words it.",
					),
					"500": openapiErrorResponse("Failed to read the run's raster files"),
				},
			},
		},
	}
}

// openapiContourSchemas describes the contour endpoint's response.
func openapiContourSchemas() map[string]any {
	return map[string]any{
		"ContourResult": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"crs", "interval", "lines"},
			"properties": map[string]any{
				"crs": map[string]any{
					"type": "string",
					"description": "The CRS the coordinates are actually in. Always present: a consumer handed " +
						"bare lines guesses WGS84, which for a metric run puts the site in the wrong hemisphere.",
				},
				"interval": map[string]any{
					"type": "number",
					"description": "The dB step the contours were generated at, echoed because the caller may " +
						"have omitted it and taken the default.",
				},
				"lines": map[string]any{
					"type":        "array",
					"description": "Every band's contours in one list, told apart by `band_name`. Empty rather than null.",
					"items":       map[string]any{"$ref": "#/components/schemas/ContourLine"},
				},
			},
		},
		"ContourLine": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"level", "band_name", "points"},
			"properties": map[string]any{
				"level":     map[string]any{"type": "number", "description": "The dB level this line traces."},
				"band_name": map[string]any{"type": "string", "description": "The raster band the line came from, e.g. `Lden`."},
				"points": map[string]any{
					"type":        "array",
					"description": "Vertices as [x, y] pairs in the response's `crs` — nested pairs here, unlike the flat batch /api/v1/transform takes.",
					"items": map[string]any{
						"type":     "array",
						"minItems": 2,
						"maxItems": 2,
						"items":    map[string]any{"type": "number"},
					},
				},
			},
		},
	}
}
