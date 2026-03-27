package httpv1

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const OpenAPIVersion = "3.1.0"

// BuildOpenAPISpec assembles the declarative OpenAPI document.
//
//nolint:funlen,maintidx
func BuildOpenAPISpec(serverURL string) map[string]any {
	server := "http://127.0.0.1:8080"
	if serverURL != "" {
		server = serverURL
	}

	return map[string]any{
		"openapi": OpenAPIVersion,
		"info": map[string]any{
			"title":       "Aconiq Local API",
			"version":     "v1",
			"description": "Local-first API used by the Aconiq frontend and local integrations.",
		},
		"servers": []map[string]any{
			{"url": server},
		},
		"paths": map[string]any{
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
							"description": "Artifact file content",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "object",
									},
								},
								"text/html": map[string]any{
									"schema": map[string]any{
										"type": "string",
									},
								},
								"text/markdown": map[string]any{
									"schema": map[string]any{
										"type": "string",
									},
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
		},
		"components": map[string]any{
			"schemas": map[string]any{
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
				"RunSummary": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"id", "scenario_id", "standard_id", "version", "status", "started_at", "finished_at", "log_path", "artifacts"},
					"properties": map[string]any{
						"id":              map[string]any{"type": "string"},
						"scenario_id":     map[string]any{"type": "string"},
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
					},
				},
				"ImportOSMRequest": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"south", "west", "north", "east"},
					"properties": map[string]any{
						"south":             map[string]any{"type": "number"},
						"west":              map[string]any{"type": "number"},
						"north":             map[string]any{"type": "number"},
						"east":              map[string]any{"type": "number"},
						"overpass_endpoint": map[string]any{"type": "string"},
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
				"ParameterDefinition": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"name", "kind", "required"},
					"properties": map[string]any{
						"name":          map[string]any{"type": "string"},
						"kind":          map[string]any{"type": "string", "enum": []string{"string", "bool", "int", "float"}},
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
					"required":             []string{"id", "description", "default_version", "versions"},
					"properties": map[string]any{
						"id":              map[string]any{"type": "string"},
						"description":     map[string]any{"type": "string"},
						"default_version": map[string]any{"type": "string"},
						"versions": map[string]any{
							"type":  "array",
							"items": map[string]any{"$ref": "#/components/schemas/VersionInfo"},
						},
					},
				},
			},
		},
	}
}

func WriteOpenAPISpec(path string, serverURL string) error {
	if path == "" {
		return errors.New("openapi output path is required")
	}

	spec := BuildOpenAPISpec(serverURL)

	encoded, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("encode openapi spec: %w", err)
	}

	encoded = append(encoded, '\n')

	err = os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return fmt.Errorf("create openapi output directory: %w", err)
	}

	err = os.WriteFile(path, encoded, 0o600)
	if err != nil {
		return fmt.Errorf("write openapi spec %s: %w", path, err)
	}

	return nil
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
