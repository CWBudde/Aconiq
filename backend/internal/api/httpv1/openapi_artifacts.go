package httpv1

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
