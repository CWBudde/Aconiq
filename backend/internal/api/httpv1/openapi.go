package httpv1

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const OpenAPIVersion = "3.1.0"

func BuildOpenAPISpec(serverURL string) map[string]any {
	server := "http://127.0.0.1:8080"
	if serverURL != "" {
		server = serverURL
	}

	return map[string]any{
		"openapi": OpenAPIVersion,
		"info": map[string]any{
			"title":       "Soundplan Local API",
			"version":     "v1",
			"description": "Local-first API used by the Soundplan frontend and local integrations.",
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
						"404": errorResponse("Project not initialized"),
						"405": methodNotAllowedResponse(),
						"500": errorResponse("Internal server error"),
					},
				},
			},
			"/api/v1/events": map[string]any{
				"get": map[string]any{
					"summary":     "Server-sent event stream",
					"operationId": "streamEvents",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "SSE stream with heartbeat and project_status events",
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
			"/api/v1/openapi.json": map[string]any{
				"get": map[string]any{
					"summary":     "OpenAPI v1 document",
					"operationId": "getOpenAPI",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "OpenAPI document",
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
						"code": map[string]any{
							"type": "string",
						},
						"message": map[string]any{
							"type": "string",
						},
						"details": map[string]any{
							"type":                 "object",
							"additionalProperties": true,
						},
						"hint": map[string]any{
							"type": "string",
						},
					},
				},
				"ErrorEnvelope": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"error"},
					"properties": map[string]any{
						"error": map[string]any{
							"$ref": "#/components/schemas/APIError",
						},
					},
				},
				"HealthResponse": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"status", "version", "time"},
					"properties": map[string]any{
						"status": map[string]any{"type": "string"},
						"version": map[string]any{
							"type": "string",
						},
						"time": map[string]any{
							"type":   "string",
							"format": "date-time",
						},
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
						"last_run": map[string]any{
							"$ref": "#/components/schemas/LastRunStatus",
						},
					},
				},
			},
		},
	}
}

func WriteOpenAPISpec(path string, serverURL string) error {
	if path == "" {
		return fmt.Errorf("openapi output path is required")
	}

	spec := BuildOpenAPISpec(serverURL)
	encoded, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("encode openapi spec: %w", err)
	}
	encoded = append(encoded, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create openapi output directory: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("write openapi spec %s: %w", path, err)
	}

	return nil
}

func methodNotAllowedResponse() map[string]any {
	return errorResponse("Method not allowed")
}

func errorResponse(description string) map[string]any {
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
