package httpv1

import (
	"github.com/aconiq/backend/internal/geo/crstransform"
)

// openapiTransformPathItems describes the coordinate projection endpoint.
func openapiTransformPathItems() map[string]any {
	return map[string]any{
		"/api/v1/transform": map[string]any{
			"post": map[string]any{
				"summary":     "Project a batch of coordinates between two CRS",
				"operationId": "transformCoordinates",
				"description": "Projects a flat, interleaved batch of coordinates. It reads no project and " +
					"writes nothing, so it answers before `aconiq init` has run. `target_crs: \"auto\"` makes " +
					"the same decision `aconiq run` makes — a geographic model goes into the ETRS89 / UTM zone " +
					"its centre falls in, a projected one is left where it is — so a client cannot place a " +
					"model where `aconiq run` would refuse to compute it. The result is a projection *of* a " +
					"model, never a source *for* one: a client that draws with it must not write the " +
					"projected coordinates back into the stored model.",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"$ref": "#/components/schemas/TransformRequest",
							},
						},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Projected batch; `target_crs` names the CRS the coordinates are actually in",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/TransformResponse",
								},
							},
						},
					},
					"400": openapiErrorResponse(
						"Malformed body, an odd number of coordinate values, or a CRS that could not be parsed " +
							"(`" + errorCodeBadRequest + "`); or a CRS pair that cannot be projected at all — a " +
							"geographic site outside ETRS89 / UTM zones 31 to 34, or a pair with no route between " +
							"them (`" + errorCodeCRSNotProjectable + "`, whose details.reason separates the two). " +
							"The message is the projector's own, worded exactly as `aconiq run` words it.",
					),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Failed to encode the projected batch"),
				},
			},
		},
	}
}

// openapiTransformSchemas describes the transform endpoint's request and response.
func openapiTransformSchemas() map[string]any {
	return map[string]any{
		"TransformRequest": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"source_crs", "coordinates"},
			"properties": map[string]any{
				"source_crs": map[string]any{
					"type":        "string",
					"description": "CRS the coordinates are in, e.g. `EPSG:4326` or `EPSG:25832`.",
				},
				"target_crs": map[string]any{
					"type": "string",
					"description": "CRS to project into. Omit it, or send `" + crstransform.AutoTarget + "`, to let the " +
						"server pick the ETRS89 / UTM zone the batch's centre falls in — the same choice " +
						"`aconiq run` makes. An explicit value always transforms, which is what makes the " +
						"inverse direction available.",
				},
				"coordinates": map[string]any{
					"type": "array",
					"description": "Flat and interleaved — x0, y0, x1, y1, … — not nested pairs. An odd number of " +
						"values is refused. This carries a model, never a receiver grid: the model is " +
						"projected first and the grid is then built in the compute CRS.",
					"items": map[string]any{"type": "number"},
				},
			},
		},
		"TransformResponse": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"source_crs", "target_crs", "applied", "coordinates"},
			"properties": map[string]any{
				"source_crs": map[string]any{"type": "string", "description": "The resolved source CRS"},
				"target_crs": map[string]any{
					"type": "string",
					"description": "The CRS the coordinates are actually in, which for an `" + crstransform.AutoTarget +
						"` request is the resolved zone rather than the string that was asked for.",
				},
				"applied": map[string]any{
					"type": "boolean",
					"description": "False only when nothing moved, and then `coordinates` are the input values " +
						"verbatim — not a round trip that happens to land close.",
				},
				"coordinates": map[string]any{
					"type":        "array",
					"description": "The projected batch, in the same flat interleaved order as the request.",
					"items":       map[string]any{"type": "number"},
				},
			},
		},
	}
}
