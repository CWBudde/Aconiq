package httpv1

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/aconiq/backend/internal/geo/crstransform"
	"github.com/aconiq/backend/internal/jsonio"
	"github.com/aconiq/backend/internal/report/contour"
)

const OpenAPIVersion = "3.1.0"

// BuildOpenAPISpec assembles the declarative OpenAPI document.
func BuildOpenAPISpec(serverURL string) map[string]any {
	spec := buildOpenAPIPaths(serverURL)
	applyTransportContract(spec)

	return spec
}

// buildOpenAPIPaths assembles the hand-built OpenAPI document. The sections are
// separate functions purely so each stays readable; the document is still one
// literal, built the same way and in the same shape.
//
// Keep all of them in this file. .golangci.yml scopes the goconst exclusion for
// the OpenAPI keyword vocabulary to openapi.go by path, so moving a section into
// a new file would surface that whole vocabulary as findings.
func buildOpenAPIPaths(serverURL string) map[string]any {
	server := "http://127.0.0.1:8080"
	if serverURL != "" {
		server = serverURL
	}

	return map[string]any{
		"openapi":    OpenAPIVersion,
		"info":       openapiInfo(),
		"servers":    openapiServers(server),
		"security":   openapiSecurity(),
		"paths":      openapiPathItems(),
		"components": openapiComponents(),
	}
}

func openapiInfo() map[string]any {
	return map[string]any{
		"title":   "Aconiq Local API",
		"version": "v1",
		"description": "Local-first API used by the Aconiq frontend and local integrations.\n\n" +
			"Every request is checked before it is routed. Its `Host` header must name a loopback " +
			"address or the host part of `aconiq serve --listen`; anything else is refused with " +
			"`forbidden_host`, which is what closes DNS rebinding. Every state-changing method must " +
			"carry a non-empty `" + ClientHeaderName + "` header — a header a CORS simple request " +
			"cannot set, so requiring it forces a preflight — and must send its body as the media " +
			"type the endpoint parses. Bearer authentication is optional and off unless the server " +
			"was started with `--api-token`.",
	}
}

func openapiServers(server string) []map[string]any {
	return []map[string]any{
		{"url": server},
	}
}

// openapiSecurity spells "optional": an empty requirement alongside the scheme
// means the token is enforced only when the server was started with one.
func openapiSecurity() []map[string]any {
	return []map[string]any{
		{},
		{"bearerAuth": []string{}},
	}
}

// openapiPathItems merges the path groups. Grouping does not affect the
// document: it is marshalled from a map, which encoding/json emits with its
// keys sorted.
func openapiPathItems() map[string]any {
	items := map[string]any{}

	for _, group := range []map[string]any{
		openapiStatusPathItems(),
		openapiRunPathItems(),
		openapiImportPathItems(),
		openapiModelPathItems(),
		openapiTransformPathItems(),
	} {
		maps.Copy(items, group)
	}

	return items
}

// openapiStatusPathItems describes the status and discovery endpoints.
func openapiStatusPathItems() map[string]any {
	return map[string]any{
		"/api/v1/health": map[string]any{
			"get": map[string]any{
				"summary":     "Health check",
				"operationId": "getHealth",
				"responses": map[string]any{
					"200": map[string]any{
						"description": "API is healthy",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/HealthResponse",
								},
							},
						},
					},
					"405": methodNotAllowedResponse(),
				},
			},
		},
		"/api/v1/project/status": map[string]any{
			"get": map[string]any{
				"summary":     "Project status",
				"operationId": "getProjectStatus",
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Loaded project status",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/ProjectStatusResponse",
								},
							},
						},
					},
					"404": openapiErrorResponse("Project not initialized"),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Internal server error"),
				},
			},
		},
		"/api/v1/standards": map[string]any{
			"get": map[string]any{
				"summary":     "List available noise standards",
				"operationId": "listStandards",
				"responses": map[string]any{
					"200": map[string]any{
						"description": "List of registered noise standards with their versions and profiles",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "array",
									"items": map[string]any{
										"$ref": "#/components/schemas/StandardDescriptor",
									},
								},
							},
						},
					},
					"405": methodNotAllowedResponse(),
					"503": openapiErrorResponse("Standards registry not configured"),
				},
			},
		},
		"/api/v1/openapi.json": map[string]any{
			"get": map[string]any{
				"summary":     "OpenAPI v1 document",
				"operationId": "getOpenAPI",
				"responses": map[string]any{
					"200": map[string]any{
						"description": "This OpenAPI document",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
								},
							},
						},
					},
					"405": methodNotAllowedResponse(),
				},
			},
		},
	}
}

// openapiRunPathItems describes the run lifecycle endpoints.
func openapiRunPathItems() map[string]any {
	items := map[string]any{
		"/api/v1/runs": map[string]any{
			"get": map[string]any{
				"summary":     "List runs (most recent first)",
				"operationId": "listRuns",
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Run summaries ordered newest first",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "array",
									"items": map[string]any{
										"$ref": "#/components/schemas/RunSummary",
									},
								},
							},
						},
					},
					"404": openapiErrorResponse("Project not initialized"),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Internal server error"),
				},
			},
			"post": map[string]any{
				"summary":     "Create and execute a run",
				"operationId": "createRun",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"$ref": "#/components/schemas/CreateRunRequest",
							},
						},
					},
				},
				"responses": map[string]any{
					"201": map[string]any{
						"description": "Created run summary",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/RunSummary",
								},
							},
						},
					},
					"400": openapiErrorResponse("Invalid run request"),
					"404": openapiErrorResponse("Project not initialized"),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Run execution failed"),
				},
			},
		},
		"/api/v1/runs/{id}/log": map[string]any{
			"get": map[string]any{
				"summary":     "Run log lines",
				"operationId": "getRunLog",
				"parameters": []map[string]any{
					{
						"name":        "id",
						"in":          "path",
						"required":    true,
						"description": "Run ID",
						"schema":      map[string]any{"type": "string"},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Log lines for the requested run",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/RunLog",
								},
							},
						},
					},
					"400": openapiErrorResponse("Missing run ID"),
					"404": openapiErrorResponse("Run not found"),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Failed to read log"),
				},
			},
		},
	}

	maps.Copy(items, openapiRunResourcePathItems())
	maps.Copy(items, openapiRunContourPathItems())
	maps.Copy(items, openapiArtifactPathItems())

	return items
}

// openapiRunResourcePathItems describes the single-run resource. Go 1.22 routing
// gives the more specific /api/v1/runs/{id}/log precedence over this pattern.
func openapiRunResourcePathItems() map[string]any {
	return map[string]any{
		"/api/v1/runs/{id}": map[string]any{
			"delete": map[string]any{
				"summary":     "Delete a run",
				"operationId": "deleteRun",
				"description": "Removes the run from the manifest, drops every artifact ref belonging to it, and " +
					"deletes `.noise/runs/{id}/`. Export bundles under `.noise/exports/` are deliberately kept on " +
					"disk — a bundle may already have been delivered — and the ones whose refs were dropped are " +
					"listed in `retained_paths`. A run that is still `pending` or `running` is refused: its " +
					"directory is being written.",
				"parameters": []map[string]any{
					{
						"name":        "id",
						"in":          "path",
						"required":    true,
						"description": "Run ID",
						"schema":      map[string]any{"type": "string"},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "The run was deleted; the body says what was removed and what was kept",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/DeleteRunResponse",
								},
							},
						},
					},
					"400": openapiErrorResponse("Missing or malformed run ID"),
					"404": openapiErrorResponse("Project not initialized, or no run with this ID (`not_found`)"),
					"405": methodNotAllowedResponse(),
					"409": openapiErrorResponse(
						"The run is still pending or running (`" + errorCodeRunNotFinished +
							"`), or it holds export bundles inside its own directory (`" + errorCodeExportInsideRun +
							"`). Nothing was removed.",
					),
					"500": openapiErrorResponse("Failed to update the manifest or remove the run directory"),
				},
			},
		},
	}
}

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

// openapiArtifactPathItems describes the artifact content and event-stream
// endpoints.
func openapiArtifactPathItems() map[string]any {
	return map[string]any{
		"/api/v1/artifacts/{id}/content": map[string]any{
			"get": map[string]any{
				"summary":     "Artifact file content",
				"operationId": "getArtifactContent",
				"parameters": []map[string]any{
					{
						"name":        "id",
						"in":          "path",
						"required":    true,
						"description": "Artifact ID",
						"schema":      map[string]any{"type": "string"},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						// The media type follows the artifact's extension; see
						// artifactContentType in handler.go, which this list
						// mirrors. The binary entries are not decoration:
						// run.result.raster_binary and the GeoTIFF/COG/GeoPackage
						// exports are bytes, and a generated client that only
						// knows application/json parses them as text.
						"description": "Artifact file content, typed by the artifact's format",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"type": "object"},
							},
							"application/geo+json": map[string]any{
								"schema": map[string]any{"type": "object"},
							},
							"text/html": map[string]any{
								"schema": map[string]any{"type": "string"},
							},
							"text/markdown": map[string]any{
								"schema": map[string]any{"type": "string"},
							},
							"text/csv": map[string]any{
								"schema": map[string]any{"type": "string"},
							},
							"text/plain": map[string]any{
								"schema": map[string]any{"type": "string"},
							},
							"application/pdf": map[string]any{
								"schema": map[string]any{"type": "string", "format": "binary"},
							},
							"image/tiff": map[string]any{
								"schema": map[string]any{"type": "string", "format": "binary"},
							},
							"application/geopackage+sqlite3": map[string]any{
								"schema": map[string]any{"type": "string", "format": "binary"},
							},
							"application/octet-stream": map[string]any{
								"schema": map[string]any{"type": "string", "format": "binary"},
							},
						},
					},
					"400": openapiErrorResponse("Missing artifact ID"),
					"404": openapiErrorResponse("Artifact not found"),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Failed to read artifact file"),
				},
			},
		},
		"/api/v1/events": map[string]any{
			"get": map[string]any{
				"summary":     "Server-sent event stream",
				"operationId": "streamEvents",
				"description": "SSE stream emitting `heartbeat` and `project_status` events. Reconnect interval is 3 s.",
				"responses": map[string]any{
					"200": map[string]any{
						"description": "SSE stream",
						"content": map[string]any{
							"text/event-stream": map[string]any{
								"schema": map[string]any{
									"type":        "string",
									"description": "SSE stream payload",
								},
							},
						},
					},
					"405": methodNotAllowedResponse(),
				},
			},
		},
	}
}

// openapiImportPathItems describes the import endpoints.
func openapiImportPathItems() map[string]any {
	return map[string]any{
		"/api/v1/import/osm": map[string]any{
			"post": map[string]any{
				"summary":     "Import OSM data for a WGS84 bounding box",
				"operationId": "importOSM",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"$ref": "#/components/schemas/ImportOSMRequest",
							},
						},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Imported GeoJSON feature collection",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
								},
							},
						},
					},
					"400": openapiErrorResponse("Invalid OSM import request"),
					"405": methodNotAllowedResponse(),
					"502": openapiErrorResponse("Overpass API request failed"),
				},
			},
		},
		"/api/v1/import/terrain": map[string]any{
			"post": map[string]any{
				"summary":     "Import a GeoTIFF terrain model",
				"operationId": "importTerrain",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"multipart/form-data": map[string]any{
							"schema": map[string]any{
								"type":     "object",
								"required": []string{"file"},
								"properties": map[string]any{
									"file": map[string]any{
										"type":   "string",
										"format": "binary",
									},
								},
							},
						},
					},
				},
				"responses": map[string]any{
					"201": map[string]any{
						"description": "Imported terrain metadata",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/TerrainInfo",
								},
							},
						},
					},
					"400": openapiErrorResponse("Invalid terrain import request"),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Failed to persist terrain artifact"),
				},
			},
		},
	}
}

// openapiModelPathItems describes the model endpoint.
func openapiModelPathItems() map[string]any {
	return map[string]any{
		"/api/v1/model": map[string]any{
			"get": map[string]any{
				"summary":     "Read the project model",
				"operationId": "getModel",
				"description": "Returns the normalized model `.noise/model/model.normalized.geojson` holds, together " +
					"with its hash. Without `crs` the stored bytes are returned untouched, so the payload is exactly " +
					"what the hash is a receipt for; with `crs` every coordinate is reprojected into that CRS first and " +
					"`crs` in the response names what the coordinates are actually in. The hash is unchanged by the " +
					"reprojection: it always describes the stored file.",
				"parameters": []map[string]any{
					{
						"name":     "crs",
						"in":       "query",
						"required": false,
						"description": "CRS to return the coordinates in, e.g. `EPSG:4326` to draw the model on a web " +
							"map. Omit it to receive the model in the project CRS, unmodified.",
						"schema": map[string]any{"type": "string"},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "The stored model",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/ModelResponse",
								},
							},
						},
					},
					"400": openapiErrorResponse("`crs` is not a recognised CRS identifier (`bad_request`)"),
					"404": openapiErrorResponse(
						"The project is not initialized (`not_found`), or it is but no model has been saved yet " +
							"(`" + errorCodeModelNotFound + "`). The two are separate codes because a client has to " +
							"be able to tell them apart.",
					),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Failed to read the stored model"),
				},
			},
			"post": map[string]any{
				"summary":     "Replace the project model",
				"operationId": "saveModel",
				"description": "Accepts a GeoJSON FeatureCollection in the v1 input schema — the same document " +
					"`aconiq import --input` takes — normalises it into the project CRS, validates it and " +
					"writes `.noise/model/model.normalized.geojson`, the model dump and the validation report, " +
					"registering the three as project artifacts. The previous model is replaced. " +
					"`aconiq run` and `POST /api/v1/runs` read the normalized model from there by default.",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"$ref": "#/components/schemas/ModelSaveRequest",
							},
						},
					},
				},
				"responses": map[string]any{
					"201": map[string]any{
						"description": "Model written; the paths are project-relative",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/ModelSaveResponse",
								},
							},
						},
					},
					"400": openapiErrorResponse(
						"Malformed body, missing `model`, or a document that is not a FeatureCollection (`bad_request`); " +
							"or a model that parsed but failed schema validation (`model_invalid`, whose details.errors " +
							"lists each finding as a ValidationIssue with its feature_id). Nothing is written in either case.",
					),
					"404": openapiErrorResponse("Project not initialized"),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Failed to persist the model"),
				},
			},
		},
	}
}

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

func openapiComponents() map[string]any {
	return map[string]any{
		"securitySchemes": openapiSecuritySchemes(),
		"schemas":         openapiSchemas(),
	}
}

func openapiSecuritySchemes() map[string]any {
	return map[string]any{
		"bearerAuth": map[string]any{
			"type":   "http",
			"scheme": "bearer",
			"description": "Opt-in. `aconiq serve --api-token` (or the ACONIQ_API_TOKEN environment " +
				"variable) makes every request present this token; started without one, the API " +
				"takes no credential and relies on the transport controls described above.",
		},
	}
}

// openapiSchemas merges the component schema groups. Grouping does not affect
// the document: it is marshalled from a map, which encoding/json emits with its
// keys sorted.
func openapiSchemas() map[string]any {
	schemas := map[string]any{}

	for _, group := range []map[string]any{
		openapiErrorSchemas(),
		openapiProjectSchemas(),
		openapiRunSchemas(),
		openapiRunDeleteSchemas(),
		openapiStandardSchemas(),
		openapiModelSchemas(),
		openapiTransformSchemas(),
		openapiContourSchemas(),
	} {
		maps.Copy(schemas, group)
	}

	return schemas
}

// openapiErrorSchemas describes the error envelope schemas.
func openapiErrorSchemas() map[string]any {
	return map[string]any{
		"APIError": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"code", "message"},
			"properties": map[string]any{
				"code":    map[string]any{"type": "string"},
				"message": map[string]any{"type": "string"},
				"details": map[string]any{
					"type":                 "object",
					"additionalProperties": true,
				},
				"hint": map[string]any{"type": "string"},
			},
		},
		"ErrorEnvelope": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"error"},
			"properties": map[string]any{
				"error": map[string]any{"$ref": "#/components/schemas/APIError"},
			},
		},
	}
}

// openapiProjectSchemas describes the health, project status and artifact schemas.
func openapiProjectSchemas() map[string]any {
	return map[string]any{
		"HealthResponse": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"status", "version", "time"},
			"properties": map[string]any{
				"status":  map[string]any{"type": "string"},
				"version": map[string]any{"type": "string"},
				"time":    map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"LastRunStatus": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"id", "status", "standard_id", "version", "started_at", "finished_at"},
			"properties": map[string]any{
				"id":          map[string]any{"type": "string"},
				"status":      map[string]any{"type": "string"},
				"context":     openapiStandardContextSchema(),
				"standard_id": map[string]any{"type": "string"},
				"version":     map[string]any{"type": "string"},
				"profile":     map[string]any{"type": "string"},
				"started_at":  map[string]any{"type": "string", "format": "date-time"},
				"finished_at": map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"ProjectStatusResponse": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"project_id", "name", "project_path", "manifest_version", "crs", "scenario_count", "run_count"},
			"properties": map[string]any{
				"project_id":       map[string]any{"type": "string"},
				"name":             map[string]any{"type": "string"},
				"project_path":     map[string]any{"type": "string"},
				"manifest_version": map[string]any{"type": "integer"},
				"crs":              map[string]any{"type": "string"},
				"scenario_count":   map[string]any{"type": "integer"},
				"run_count":        map[string]any{"type": "integer"},
				"last_run":         map[string]any{"$ref": "#/components/schemas/LastRunStatus"},
				"model":            map[string]any{"$ref": "#/components/schemas/ProjectModelStatus"},
			},
		},
		"ProjectModelStatus": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"hash", "updated_at"},
			"description": "The saved model's receipt. Absent until a model has been saved. A client that kept " +
				"the hash POST /api/v1/model returned compares the two as strings to learn whether its local draft " +
				"is still the project's model, without fetching anything.",
			"properties": map[string]any{
				"hash":       openapiModelHashSchema(),
				"updated_at": map[string]any{"type": "string", "format": "date-time", "description": "When the model artifact was last written"},
			},
		},
		"ArtifactRef": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"id", "kind", "path", "created_at"},
			"properties": map[string]any{
				"id":         map[string]any{"type": "string"},
				"kind":       map[string]any{"type": "string"},
				"path":       map[string]any{"type": "string"},
				"created_at": map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"TerrainInfo": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"bounds", "pixel_size", "grid_size"},
			"properties": map[string]any{
				"bounds": map[string]any{
					"type":        "array",
					"minItems":    4,
					"maxItems":    4,
					"description": "Bounding box [min_x, min_y, max_x, max_y]",
					"items":       map[string]any{"type": "number"},
				},
				"pixel_size": map[string]any{
					"type":        "array",
					"minItems":    2,
					"maxItems":    2,
					"description": "Pixel size [width, height] in CRS units",
					"items":       map[string]any{"type": "number"},
				},
				"grid_size": map[string]any{
					"type":        "array",
					"minItems":    2,
					"maxItems":    2,
					"description": "Raster dimensions [width, height]",
					"items":       map[string]any{"type": "integer"},
				},
			},
		},
	}
}

// openapiRunSchemas describes the run request and result schemas.
func openapiRunSchemas() map[string]any {
	return map[string]any{
		"RunSummary": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"id", "scenario_id", "standard_id", "version", "status", "started_at", "finished_at", "log_path", "artifacts"},
			"properties": map[string]any{
				"id":              map[string]any{"type": "string"},
				"scenario_id":     map[string]any{"type": "string"},
				"context":         openapiStandardContextSchema(),
				"standard_id":     map[string]any{"type": "string"},
				"version":         map[string]any{"type": "string"},
				"profile":         map[string]any{"type": "string"},
				"receiver_mode":   map[string]any{"type": "string"},
				"receiver_set_id": map[string]any{"type": "string"},
				"status": map[string]any{
					"type": "string",
					"enum": []string{"pending", "running", "completed", "failed"},
				},
				"started_at":  map[string]any{"type": "string", "format": "date-time"},
				"finished_at": map[string]any{"type": "string", "format": "date-time"},
				"log_path":    map[string]any{"type": "string"},
				"artifacts": map[string]any{
					"type":  "array",
					"items": map[string]any{"$ref": "#/components/schemas/ArtifactRef"},
				},
			},
		},
		"RunLog": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"run_id", "lines"},
			"properties": map[string]any{
				"run_id": map[string]any{"type": "string"},
				"lines": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
			},
		},
		"CreateRunRequest": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"scenario_id":      map[string]any{"type": "string"},
				"standard_id":      map[string]any{"type": "string"},
				"standard_version": map[string]any{"type": "string"},
				"standard_profile": map[string]any{"type": "string"},
				"model_path":       map[string]any{"type": "string"},
				"receiver_mode":    map[string]any{"type": "string", "enum": []string{"auto-grid", "custom"}},
				"params": map[string]any{
					"type":                 "object",
					"additionalProperties": map[string]any{"type": "string"},
				},
				"input_paths": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"experimental": map[string]any{
					"type":        "boolean",
					"default":     false,
					"description": "Required for scaffold-tier standards: they carry no normative coefficients, their base levels are invented and they have no octave bands, so a run must acknowledge that before it may emit levels. Requests without it are rejected with error code experimental_opt_in_required.",
				},
			},
		},
		"ImportOSMRequest": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"south", "west", "north", "east"},
			"properties": map[string]any{
				"south": map[string]any{"type": "number"},
				"west":  map[string]any{"type": "number"},
				"north": map[string]any{"type": "number"},
				"east":  map[string]any{"type": "number"},
				"overpass_endpoint": map[string]any{
					"type":   "string",
					"format": "uri",
					"description": "Optional Overpass server. It arrives in a request body, so it is " +
						"constrained rather than trusted: https only, and only a known Overpass host. " +
						"Anything else is refused with error code `overpass_endpoint_not_allowed`, whose " +
						"details.allowed_hosts lists what is accepted. Omit it to use the default server.",
					"examples": allowedOverpassEndpointURLs(),
				},
			},
		},
	}
}

// openapiRunDeleteSchemas describes what a run delete answers with.
func openapiRunDeleteSchemas() map[string]any {
	return map[string]any{
		"DeleteRunResponse": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"run_id", "removed_paths", "retained_paths"},
			"properties": map[string]any{
				"run_id": map[string]any{"type": "string"},
				"removed_paths": map[string]any{
					"type":        "array",
					"description": "Project-relative paths that were deleted. Always present, empty rather than null.",
					"items":       map[string]any{"type": "string"},
				},
				"retained_paths": map[string]any{
					"type": "array",
					"description": "Files whose manifest refs were dropped but whose bytes were deliberately left " +
						"in place — export bundles. Always present, empty rather than null.",
					"items": map[string]any{"type": "string"},
				},
			},
		},
	}
}

// openapiStandardSchemas describes the standards registry schemas.
func openapiStandardSchemas() map[string]any {
	return map[string]any{
		"ParameterDefinition": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"name", "kind", "required"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"kind": map[string]any{"type": "string", "enum": []string{"string", "bool", "int", "float"}},
				"unit": map[string]any{
					"type":        "string",
					"description": "Physical unit of the value as a short symbol (m, km/h, dB, dB/km, 1/h, 1/km, %, °C, °). Absent when the parameter is dimensionless or not numeric.",
				},
				"required":      map[string]any{"type": "boolean"},
				"default_value": map[string]any{"type": "string"},
				"description":   map[string]any{"type": "string"},
				"enum": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"min": map[string]any{"type": "number"},
				"max": map[string]any{"type": "number"},
			},
		},
		"ProfileInfo": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"name", "supported_source_types", "supported_indicators", "parameters"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"supported_source_types": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"supported_indicators": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"parameters": map[string]any{
					"type":  "array",
					"items": map[string]any{"$ref": "#/components/schemas/ParameterDefinition"},
				},
			},
		},
		"VersionInfo": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"name", "default_profile", "profiles"},
			"properties": map[string]any{
				"name":            map[string]any{"type": "string"},
				"default_profile": map[string]any{"type": "string"},
				"profiles": map[string]any{
					"type":  "array",
					"items": map[string]any{"$ref": "#/components/schemas/ProfileInfo"},
				},
			},
		},
		"StandardDescriptor": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			// context has no omitempty on descriptorjson.Standard, so the key is always
			// present and a strict consumer may rely on it.
			"required": []string{"context", "id", "description", evidenceTierField, "default_version", "versions"},
			"properties": map[string]any{
				"context":     openapiStandardContextSchema(),
				"id":          map[string]any{"type": "string"},
				"description": map[string]any{"type": "string"},
				evidenceTierField: map[string]any{
					"type": "string",
					"description": "How far this module's output may be trusted. " +
						"`normative` implements the named standard's tables and equations within the boundary declared under docs/conformance/; " +
						"`preview` has sound logic but consumes preview-grade input levels; " +
						"`scaffold` carries no normative coefficients and its levels are invented; " +
						"`test-fixture` is a non-normative demonstrator. Only `normative` output is usable for assessment.",
					"enum": []string{"normative", "preview", "scaffold", "test-fixture"},
				},
				"default_version": map[string]any{"type": "string"},
				"versions": map[string]any{
					"type":  "array",
					"items": map[string]any{"$ref": "#/components/schemas/VersionInfo"},
				},
			},
		},
	}
}

// openapiModelSchemas describes the model endpoint's request and response.
func openapiModelSchemas() map[string]any {
	return map[string]any{
		"ModelSaveRequest": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"model"},
			"properties": map[string]any{
				"crs": map[string]any{
					"type": "string",
					"description": "CRS of the model's coordinates, e.g. `EPSG:4326` for coordinates drawn on a " +
						"web map. Like `aconiq import --input-crs`: when it differs from the project CRS every " +
						"coordinate is reprojected; when omitted the coordinates are taken to be in the project CRS already.",
				},
				"model": map[string]any{
					"type":        "object",
					"description": "GeoJSON FeatureCollection in the v1 input schema (docs/geojson-schema-v1.md): features carry `kind` = source | building | barrier | receiver | calc-area | ground-zone.",
				},
			},
		},
		"ModelSaveResponse": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"normalized_path", "dump_path", "validation_report_path", "feature_count", "hash", "warnings"},
			"properties": map[string]any{
				"normalized_path":        map[string]any{"type": "string", "description": "Project-relative path of the normalized GeoJSON"},
				"dump_path":              map[string]any{"type": "string", "description": "Project-relative path of the model dump"},
				"validation_report_path": map[string]any{"type": "string", "description": "Project-relative path of the validation report"},
				"feature_count":          map[string]any{"type": "integer"},
				"hash":                   openapiModelHashSchema(),
				"warnings": map[string]any{
					"type":        "array",
					"description": "Validation warnings. The model was written despite them.",
					"items":       map[string]any{"$ref": "#/components/schemas/ValidationIssue"},
				},
			},
		},
		"ModelResponse": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"crs", "project_crs", "hash", "feature_count", "model"},
			"properties": map[string]any{
				"crs": map[string]any{
					"type": "string",
					"description": "The CRS the returned coordinates are in — the requested one, or the project " +
						"CRS when none was requested. Stated rather than inferred.",
				},
				"project_crs":   map[string]any{"type": "string", "description": "The project's own CRS"},
				"hash":          openapiModelHashSchema(),
				"feature_count": map[string]any{"type": "integer"},
				"model": map[string]any{
					"type":        "object",
					"description": "The normalized GeoJSON FeatureCollection. Returned verbatim from disk unless `crs` asked for a reprojection.",
				},
			},
		},
		"ValidationIssue": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"code", "message"},
			"properties": map[string]any{
				"code":       map[string]any{"type": "string"},
				"feature_id": map[string]any{"type": "string", "description": "The feature the finding is about; absent for model-wide findings"},
				"message":    map[string]any{"type": "string"},
			},
		},
	}
}

func WriteOpenAPISpec(path string, serverURL string) error {
	if path == "" {
		return errors.New("openapi output path is required")
	}

	spec := BuildOpenAPISpec(serverURL)

	encoded, err := jsonio.Marshal(spec)
	if err != nil {
		return fmt.Errorf("encode openapi spec: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(path), 0o750)
	if err != nil {
		return fmt.Errorf("create openapi output directory: %w", err)
	}

	err = os.WriteFile(path, encoded, 0o600)
	if err != nil {
		return fmt.Errorf("write openapi spec %s: %w", path, err)
	}

	return nil
}

// applyTransportContract stamps the security middleware's answers onto every
// operation.
//
// The middleware is cross-cutting — it runs before routing, so its refusals can
// appear on any path — and documenting it per operation by hand is exactly the
// kind of duplication that drifts. Adding it here the way the middleware adds it
// to a request keeps handler.go and this document in step by construction.
func applyTransportContract(spec map[string]any) {
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		return
	}

	for _, item := range paths {
		operations, ok := item.(map[string]any)
		if !ok {
			continue
		}

		for method, operation := range operations {
			typed, ok := operation.(map[string]any)
			if !ok {
				continue
			}

			applyOperationTransportContract(typed, method)
		}
	}
}

func applyOperationTransportContract(operation map[string]any, method string) {
	responses, ok := operation["responses"].(map[string]any)
	if !ok {
		responses = map[string]any{}
		operation["responses"] = responses
	}

	responses["401"] = openapiErrorResponse(
		"Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token.",
	)
	responses["403"] = openapiErrorResponse(
		"Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing " +
			"request arrived without the " + ClientHeaderName + " header (`client_header_required`), or the project " +
			"manifest named a path outside the project root (`forbidden_path`).",
	)

	if method == "get" {
		return
	}

	responses["413"] = openapiErrorResponse("Request body exceeds this endpoint's limit (`request_too_large`)")

	// 415 comes from requireContentType, which only an operation that parses a
	// body ever calls. Stamping it on a bodyless DELETE would document a refusal
	// the server cannot produce.
	if _, hasBody := operation["requestBody"]; hasBody {
		responses["415"] = openapiErrorResponse(
			"Request body was not sent as the media type this endpoint parses (`unsupported_media_type`)",
		)
	}

	parameters, _ := operation["parameters"].([]map[string]any)
	operation["parameters"] = append(parameters, clientHeaderParameter())
}

// clientHeaderParameter documents the custom header required on every
// state-changing method.
func clientHeaderParameter() map[string]any {
	return map[string]any{
		"name":     ClientHeaderName,
		"in":       "header",
		"required": true,
		"description": "Any non-empty value. A CORS simple request cannot set a custom header, so requiring " +
			"one forces a preflight, which the origin allowlist then answers or does not. The value is never read.",
		"schema": map[string]any{"type": "string", "minLength": 1},
	}
}

// allowedOverpassEndpointURLs renders the Overpass allowlist as the URLs a
// caller may actually send, so the document does not restate the list by hand.
func allowedOverpassEndpointURLs() []string {
	urls := make([]string, 0, len(allowedOverpassHosts))
	for _, host := range allowedOverpassHosts {
		urls = append(urls, "https://"+host+"/api/interpreter")
	}

	return urls
}

// openapiStandardContextSchema describes the standard's assessment context, the
// member `StandardRef.Context` and `StandardDescriptor.Context` travel under.
// It is one function because the same member appears on three schemas, and a
// consumer that switches on it must read the same enum everywhere.
func openapiStandardContextSchema() map[string]any {
	return map[string]any{
		"type": "string",
		"description": "Which assessment question the standard answers: `planning` for an individual " +
			"project's approval case, `mapping` for area-wide strategic noise mapping. The two are not " +
			"interchangeable, so a result carries the context it was produced under.",
		"enum": []string{"planning", "mapping"},
	}
}

// openapiModelHashSchema describes the model receipt. The same value appears on
// three schemas, and the one thing every consumer must understand about it is
// that it is never recomputed client-side.
func openapiModelHashSchema() map[string]any {
	return map[string]any{
		"type":    "string",
		"pattern": "^[0-9a-f]{64}$",
		"description": "SHA-256 of the stored normalized model, as bare lowercase hex — the spelling " +
			"provenance.json uses for input hashes. It is a receipt issued by the server: keep the value you " +
			"were handed and compare it as a string. Do not recompute it from a model you hold, because a " +
			"re-serialised model is not the same bytes.",
	}
}

func methodNotAllowedResponse() map[string]any {
	return openapiErrorResponse("Method not allowed")
}

func openapiErrorResponse(description string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{
					"$ref": "#/components/schemas/ErrorEnvelope",
				},
			},
		},
	}
}
